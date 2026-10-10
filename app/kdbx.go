//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//
// getEntry and encodeQuery follow internal/cmd/keepass/export.go of
// github.com/passbolt/go-passbolt-cli (MIT License, Copyright (c) 2021
// Samuel Lorch), extended here with the folder tree, KDBX 4 and key files.
//

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// Argon2d settings in the range KeePassXC uses by default; the library
// default (1 MiB) is too weak for a file that is stored in a cloud.
const (
	kdfMemory      = 64 * 1024 * 1024
	kdfIterations  = 10
	kdfParallelism = 2
	maxFolderDepth = 64
)

func credentials(cfg *Config) (*gokeepasslib.DBCredentials, error) {
	if cfg.KeyFile == "" {
		return gokeepasslib.NewPasswordCredentials(cfg.KdbxPassword), nil
	}
	creds, err := gokeepasslib.NewPasswordAndKeyDataCredentials(cfg.KdbxPassword, []byte(cfg.KeyFile))
	if err != nil {
		return nil, fmt.Errorf("reading the key file: %w", err)
	}
	return creds, nil
}

// buildDatabase decrypts everything the account can read and arranges the
// entries in the Passbolt folder tree. Resources that cannot be decrypted are
// reported by ID (never by name) and left out.
func buildDatabase(ctx context.Context, client *api.Client, res *Result) (*gokeepasslib.Database, error) {
	folders, err := client.GetFolders(ctx, nil)
	if err != nil {
		return nil, explain("listing folders", err)
	}
	resources, err := client.GetResources(ctx, &api.GetResourcesOptions{
		ContainSecret:       true,
		ContainResourceType: true,
		ContainTags:         true,
	})
	if err != nil {
		return nil, explain("listing resources", err)
	}
	res.Folders, res.Resources = len(folders), len(resources)
	logf("decrypting %d resources in %d folders", len(resources), len(folders))

	byID := map[string]api.Folder{}
	for _, f := range folders {
		byID[f.ID] = f
	}
	childFolders := map[string][]api.Folder{}
	for _, f := range folders {
		parent := f.FolderParentID
		if _, ok := byID[parent]; !ok {
			parent = "" // parent not visible to this account: show at the top
		}
		childFolders[parent] = append(childFolders[parent], f)
	}
	entries := map[string][]gokeepasslib.Entry{}
	for _, r := range resources {
		if len(r.Secrets) == 0 {
			res.Skipped = append(res.Skipped, r.ID)
			continue
		}
		entry, err := getEntry(client, r, r.Secrets[0], r.ResourceType)
		if err != nil {
			logf("skipping resource %s: %v", r.ID, err)
			res.Skipped = append(res.Skipped, r.ID)
			continue
		}
		parent := r.FolderParentID
		if _, ok := byID[parent]; !ok {
			parent = ""
		}
		entries[parent] = append(entries[parent], *entry)
		res.Entries++
	}

	visited := map[string]bool{}
	var build func(id, name string, depth int) (gokeepasslib.Group, error)
	build = func(id, name string, depth int) (gokeepasslib.Group, error) {
		g := gokeepasslib.NewGroup()
		g.Name = name
		if depth > maxFolderDepth {
			return g, errors.New("the folder tree is nested too deeply")
		}
		g.Entries = entries[id]
		sort.SliceStable(g.Entries, func(i, j int) bool {
			return strings.ToLower(g.Entries[i].GetTitle()) < strings.ToLower(g.Entries[j].GetTitle())
		})
		kids := childFolders[id]
		sort.SliceStable(kids, func(i, j int) bool { return strings.ToLower(kids[i].Name) < strings.ToLower(kids[j].Name) })
		for _, k := range kids {
			if visited[k.ID] {
				continue // defensive: a cycle in the folder data
			}
			visited[k.ID] = true
			sub, err := build(k.ID, k.Name, depth+1)
			if err != nil {
				return g, err
			}
			g.Groups = append(g.Groups, sub)
		}
		return g, nil
	}
	root, err := build("", "Passbolt", 0)
	if err != nil {
		return nil, err
	}

	db := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion40())
	kdf := db.Header.FileHeaders.KdfParameters
	kdf.Memory, kdf.Iterations, kdf.Parallelism = kdfMemory, kdfIterations, kdfParallelism
	db.Content.Meta.DatabaseName = "Passbolt Export"
	db.Content.Root = &gokeepasslib.RootData{Groups: []gokeepasslib.Group{root}}
	return db, nil
}

// writeDatabase encrypts db to path and returns size and SHA-256.
func writeDatabase(db *gokeepasslib.Database, creds *gokeepasslib.DBCredentials, path string) (int64, string, error) {
	db.Credentials = creds
	if err := db.LockProtectedEntries(); err != nil {
		return 0, "", fmt.Errorf("protecting entries: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, "", fmt.Errorf("creating the export file: %w", err)
	}
	h := sha256.New()
	cw := &countWriter{w: io.MultiWriter(f, h)}
	if err := gokeepasslib.NewEncoder(cw).Encode(db); err != nil {
		f.Close()
		return 0, "", fmt.Errorf("encoding the KeePass file: %w", err)
	}
	if err := f.Close(); err != nil {
		return 0, "", fmt.Errorf("writing the export file: %w", err)
	}
	return cw.n, hex.EncodeToString(h.Sum(nil)), nil
}

// verifyDatabase opens the written file again with the same credentials and
// counts the entries: the restore test of every run.
func verifyDatabase(path string, creds *gokeepasslib.DBCredentials) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	db := gokeepasslib.NewDatabase()
	db.Credentials = creds
	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		return 0, fmt.Errorf("the written file cannot be opened again: %w", err)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		return 0, fmt.Errorf("the written file cannot be unlocked again: %w", err)
	}
	n := 0
	var count func(g gokeepasslib.Group)
	count = func(g gokeepasslib.Group) {
		n += len(g.Entries)
		for _, s := range g.Groups {
			count(s)
		}
	}
	for _, g := range db.Content.Root.Groups {
		count(g)
	}
	return n, nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func getEntry(client *api.Client, resource api.Resource, secret api.Secret, rType api.ResourceType) (*gokeepasslib.Entry, error) {
	_, metadata, secretFields, err := helper.GetResourceFieldMaps(client, resource, secret, rType, true)
	if err != nil {
		return nil, err
	}

	name := helper.GetStringField(metadata, "name")
	username := helper.GetStringField(metadata, "username")
	uri := helper.GetStringField(metadata, "uri")
	password := helper.GetStringField(secretFields, "password")
	description := helper.GetStringField(metadata, "description")
	if description == "" {
		// v5 types keep the description in the encrypted part
		description = helper.GetStringField(secretFields, "description")
	}

	entry := gokeepasslib.NewEntry()
	entry.Values = append(entry.Values,
		gokeepasslib.ValueData{Key: "Title", Value: gokeepasslib.V{Content: name}},
		gokeepasslib.ValueData{Key: "UserName", Value: gokeepasslib.V{Content: username}},
		gokeepasslib.ValueData{Key: "URL", Value: gokeepasslib.V{Content: uri}},
		gokeepasslib.ValueData{Key: "Password", Value: gokeepasslib.V{Content: password, Protected: w.NewBoolWrapper(true)}},
		gokeepasslib.ValueData{Key: "Notes", Value: gokeepasslib.V{Content: description}},
	)

	if totpRaw, ok := secretFields["totp"].(map[string]any); ok {
		if secretKey, _ := totpRaw["secret_key"].(string); secretKey != "" {
			algorithm, _ := totpRaw["algorithm"].(string)
			var digits, period int
			if d, ok := totpRaw["digits"].(float64); ok {
				digits = int(d)
			}
			if p, ok := totpRaw["period"].(float64); ok {
				period = int(p)
			}
			v := url.Values{}
			v.Set("secret", secretKey)
			v.Set("period", strconv.Itoa(period))
			v.Set("algorithm", algorithm)
			v.Set("digits", strconv.Itoa(digits))
			issuer := uri
			if issuer == "" {
				issuer = name
			}
			v.Set("issuer", issuer)
			account := username
			if account == "" {
				account = name
			}
			u := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + issuer + ":" + account, RawQuery: encodeQuery(v)}
			entry.Values = append(entry.Values, gokeepasslib.ValueData{Key: "otp", Value: gokeepasslib.V{Content: u.String(), Protected: w.NewBoolWrapper(true)}})
		}
	}

	for _, cf := range helper.ParseCustomFields(metadata, secretFields) {
		if cf.Name == "" || isStandardKey(cf.Name) {
			continue
		}
		entry.Values = append(entry.Values, gokeepasslib.ValueData{
			Key:   cf.Name,
			Value: gokeepasslib.V{Content: cf.Value, Protected: w.NewBoolWrapper(true)},
		})
	}
	return &entry, nil
}

// isStandardKey keeps a custom field from overwriting a standard KeePass field.
func isStandardKey(k string) bool {
	switch k {
	case "Title", "UserName", "URL", "Password", "Notes", "otp":
		return true
	}
	return false
}

// encodeQuery is url.Values.Encode with %20 instead of + for spaces, which
// some authenticator apps need.
func encodeQuery(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for _, k := range keys {
		for _, val := range v[k] {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(url.PathEscape(k))
			buf.WriteByte('=')
			buf.WriteString(url.PathEscape(val))
		}
	}
	return buf.String()
}
