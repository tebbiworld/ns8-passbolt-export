//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// workDir is a tmpfs inside the container: the clear-text never exists
// anywhere, and the encrypted file only in memory until it is uploaded.
func workDir() string {
	if d := os.Getenv("PBEXPORT_WORKDIR"); d != "" {
		return d
	}
	return "/work"
}

func runExport(ctx context.Context, cfg *Config, res *Result) error {
	if err := cfg.validateExport(); err != nil {
		return err
	}
	creds, err := credentials(cfg)
	if err != nil {
		return err
	}

	client, err := login(ctx, cfg, res)
	if err != nil {
		return err
	}
	db, err := buildDatabase(ctx, client, res)
	logout(client)
	if err != nil {
		return err
	}

	path := filepath.Join(workDir(), cfg.FileName)
	defer removeQuietly(path)
	size, sum, err := writeDatabase(db, creds, path)
	if err != nil {
		return err
	}
	res.FileName, res.Size, res.SHA256 = cfg.FileName, size, sum

	n, err := verifyDatabase(path, creds)
	if err != nil {
		return err
	}
	if n != res.Entries {
		return fmt.Errorf("the written file holds %d entries instead of %d", n, res.Entries)
	}
	res.Verified = true
	logf("written and verified: %d entries, %d bytes", n, size)

	if err := upload(ctx, path, cfg.Remote, cfg.FileName, size); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	res.Uploaded = true

	pruned, kept, err := prune(ctx, cfg.Remote, cfg.FileName, cfg.Retention)
	res.Pruned, res.Kept = pruned, kept
	if err != nil {
		return fmt.Errorf("the export was uploaded, but removing old copies failed: %w", err)
	}
	return nil
}
