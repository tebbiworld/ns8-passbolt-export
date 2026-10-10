//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// A KeePass 2.0 XML key file as the module UI generates it.
func testKeyFile(key []byte) string {
	sum := sha256.Sum256(key)
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<KeyFile><Meta><Version>2.0</Version></Meta><Key><Data Hash="%X">%X</Data></Key></KeyFile>
`, sum[:4], key)
}

func testDB() *gokeepasslib.Database {
	e := gokeepasslib.NewEntry()
	e.Values = append(e.Values,
		gokeepasslib.ValueData{Key: "Title", Value: gokeepasslib.V{Content: "mail"}},
		gokeepasslib.ValueData{Key: "Password", Value: gokeepasslib.V{Content: "s3cret", Protected: w.NewBoolWrapper(true)}},
	)
	sub := gokeepasslib.NewGroup()
	sub.Name = "Team"
	sub.Entries = []gokeepasslib.Entry{e, e}
	root := gokeepasslib.NewGroup()
	root.Name = "Passbolt"
	root.Entries = []gokeepasslib.Entry{e}
	root.Groups = []gokeepasslib.Group{sub}
	db := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion40())
	db.Header.FileHeaders.KdfParameters.Memory = 1024 * 1024 // keep the test fast
	db.Content.Root = &gokeepasslib.RootData{Groups: []gokeepasslib.Group{root}}
	return db
}

func TestWriteAndVerifyWithKeyFile(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i * 7)
	}
	cfg := &Config{KdbxPassword: "correct horse battery", KeyFile: testKeyFile(key)}
	creds, err := credentials(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "passbolt-export-2026-10-10_031500.kdbx")
	size, sum, err := writeDatabase(testDB(), creds, path)
	if err != nil || size == 0 || len(sum) != 64 {
		t.Fatalf("write: %v %d %q", err, size, sum)
	}
	n, err := verifyDatabase(path, creds)
	if err != nil || n != 3 {
		t.Fatalf("verify: %v, %d entries", err, n)
	}

	// The password alone must not open a file protected by a key file.
	if _, err := verifyDatabase(path, gokeepasslib.NewPasswordCredentials(cfg.KdbxPassword)); err == nil {
		t.Fatal("file opened without the key file")
	}
}

func TestBrokenKeyFileIsRejected(t *testing.T) {
	cfg := &Config{KdbxPassword: "correct horse battery", KeyFile: strings.Replace(testKeyFile(make([]byte, 32)), "Hash=\"", "Hash=\"0", 1)}
	if _, err := credentials(cfg); err == nil {
		t.Fatal("key file with a wrong hash accepted")
	}
}

func TestFileNamePattern(t *testing.T) {
	good := []string{"passbolt-export-2026-10-10_031500.kdbx", "a-2026-01-01_000000.kdbx"}
	bad := []string{"passbolt-export-2026-10-10.kdbx", "../x-2026-10-10_031500.kdbx", "x-2026-10-10_031500.kdbx.bak",
		"notes.kdbx", "-2026-10-10_031500.kdbx", "x y-2026-10-10_031500.kdbx"}
	for _, n := range good {
		if _, ok := prefixOf(n); !ok {
			t.Errorf("%q rejected", n)
		}
	}
	for _, n := range bad {
		if _, ok := prefixOf(n); ok {
			t.Errorf("%q accepted", n)
		}
	}
}

func TestValidate(t *testing.T) {
	base := Config{ServerURL: "https://passbolt.example.com", PrivateKey: "-----BEGIN PGP PRIVATE KEY BLOCK-----", Passphrase: "x"}
	ok := base
	ok.Remote = "target:Backups/Passbolt"
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(c *Config){
		"http":       func(c *Config) { c.ServerURL = "http://passbolt.example.com" },
		"userinfo":   func(c *Config) { c.ServerURL = "https://u:p@passbolt.example.com" },
		"no key":     func(c *Config) { c.PrivateKey = "x" },
		"no pass":    func(c *Config) { c.Passphrase = "" },
		"dotdot":     func(c *Config) { c.Remote = "target:../x" },
		"other":      func(c *Config) { c.Remote = "other:x" },
		"flag":       func(c *Config) { c.Remote = "target:x --config=/etc/passwd" },
		"half token": func(c *Config) { c.VerifyToken = "x" },
	}
	for name, mutate := range cases {
		c := base
		mutate(&c)
		if err := c.validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSealRoundTrip(t *testing.T) {
	out := seal(strings.NewReader(`{"password":"admin pw","plaintext":"geheim"}`))
	if !out.OK {
		t.Fatal(out.Error)
	}
	dec := func(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }
	key, _ := pbkdf2.Key(sha256.New, "admin pw", dec(out.Salt), out.Iterations, 32)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, dec(out.Nonce), dec(out.Ciphertext), nil)
	if err != nil || string(plain) != "geheim" {
		t.Fatalf("open: %v %q", err, plain)
	}
	if bad := seal(strings.NewReader(`{"password":"","plaintext":"x"}`)); bad.OK {
		t.Fatal("empty password accepted")
	}
}
