//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// caFile is set when a private CA is configured: rclone then trusts it for
// the upload target as well (e.g. an internal S3 or WebDAV server). rclone's
// --ca-cert replaces the system roots, so the bundle holds both.
var caFile string

const systemBundle = "/etc/ssl/certs/ca-certificates.crt"

func useCA(pem string) error {
	if pem == "" {
		return nil
	}
	bundle, err := os.ReadFile(systemBundle)
	if err != nil {
		return fmt.Errorf("reading the system CA bundle: %w", err)
	}
	bundle = append(append(bundle, '\n'), []byte(pem)...)
	path := filepath.Join(workDir(), "ca.pem")
	if err := os.WriteFile(path, bundle, 0o600); err != nil {
		return fmt.Errorf("writing the CA certificate: %w", err)
	}
	caFile = path
	return nil
}

// rclone runs the bundled rclone with the module's configuration file
// (RCLONE_CONFIG, set by the container). Arguments are passed as a list,
// never through a shell.
func rclone(ctx context.Context, args ...string) ([]byte, error) {
	args = append(args, "--retries", "3", "--low-level-retries", "10", "--stats", "0")
	if caFile != "" {
		args = append(args, "--ca-cert", caFile)
	}
	cmd := exec.CommandContext(ctx, "rclone", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 600 {
			msg = msg[len(msg)-600:]
		}
		return nil, fmt.Errorf("rclone %s: %v: %s", args[0], err, msg)
	}
	return stdout.Bytes(), nil
}

// probeTarget creates the target folder if needed and lists it.
func probeTarget(ctx context.Context, remote string) error {
	if _, err := rclone(ctx, "mkdir", remote); err != nil {
		return fmt.Errorf("the upload target is not reachable: %w", err)
	}
	if _, err := listTarget(ctx, remote); err != nil {
		return fmt.Errorf("the upload target cannot be listed: %w", err)
	}
	return nil
}

func listTarget(ctx context.Context, remote string) ([]string, error) {
	out, err := rclone(ctx, "lsjson", "--files-only", "--no-modtime", "--no-mimetype", remote)
	if err != nil {
		return nil, err
	}
	var items []struct {
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("reading the target listing: %w", err)
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names, nil
}

func upload(ctx context.Context, local, remote, name string, size int64) error {
	dst := remote + "/" + name
	if strings.HasSuffix(remote, ":") {
		dst = remote + name
	}
	if _, err := rclone(ctx, "copyto", local, dst); err != nil {
		return err
	}
	// rclone compares hashes where the backend supports it; the size check
	// also covers the others.
	out, err := rclone(ctx, "lsjson", "--files-only", "--no-mimetype", dst)
	if err != nil {
		return fmt.Errorf("checking the uploaded file: %w", err)
	}
	var items []struct {
		Size int64 `json:"Size"`
	}
	if err := json.Unmarshal(out, &items); err != nil || len(items) != 1 || items[0].Size != size {
		return fmt.Errorf("the uploaded file is missing or has the wrong size")
	}
	return nil
}

// prune deletes the oldest exports with the same prefix beyond keep. Only
// file names this tool writes are considered, everything else on the target
// is left alone.
func prune(ctx context.Context, remote, current string, keep int) (pruned []string, kept int, err error) {
	prefix, _ := prefixOf(current)
	names, err := listTarget(ctx, remote)
	if err != nil {
		return nil, 0, err
	}
	var ours []string
	for _, n := range names {
		if p, ok := prefixOf(n); ok && p == prefix {
			ours = append(ours, n)
		}
	}
	sort.Strings(ours) // the timestamp in the name sorts chronologically
	for len(ours) > keep {
		victim := ours[0]
		if victim == current {
			break
		}
		dst := remote + "/" + victim
		if strings.HasSuffix(remote, ":") {
			dst = remote + victim
		}
		if _, err := rclone(ctx, "deletefile", dst); err != nil {
			return pruned, len(ours), err
		}
		pruned = append(pruned, victim)
		ours = ours[1:]
	}
	return pruned, len(ours), nil
}

func removeQuietly(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logf("could not remove %s: %v", path, err)
	}
}
