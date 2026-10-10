//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

// pbexport exports the Passbolt resources one account can decrypt into an
// encrypted KeePass database (KDBX 4) and uploads it with rclone.
//
// It is started by the NS8 module in a short-lived container. The settings,
// including all secrets, arrive as one JSON document on stdin, so nothing
// sensitive shows up in the process list, in the container definition or in
// the environment. The result is one JSON document on stdout; progress and
// errors go to stderr and never contain secret values.
//
// Commands:
//
//	check   log in, pin the server key on first use, count what the account
//	        can see and probe the upload target
//	run     export, verify the written file by decrypting it again, upload it
//	        and prune old copies beyond the retention count
//	seal    encrypt the emergency sheet for the browser with the admin password
//	pdf     the emergency sheet as AES-256 encrypted PDF, sealed the same way
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const maxConfigSize = 1 << 20 // 1 MiB is far more than a key plus settings need

// Config is the JSON document read from stdin.
type Config struct {
	ServerURL      string `json:"server_url"`
	CAPEM          string `json:"ca_pem"`
	PrivateKey     string `json:"private_key"`
	Passphrase     string `json:"passphrase"`
	TOTPSecret     string `json:"totp_secret"`
	VerifyToken    string `json:"verify_token"`
	VerifyEncToken string `json:"verify_enc_token"`

	KdbxPassword string `json:"kdbx_password"`
	KeyFile      string `json:"keyfile"`

	// Remote is an rclone path, e.g. "target:Passbolt". Empty means: do not
	// touch the target (check only the Passbolt side).
	Remote    string `json:"remote"`
	FileName  string `json:"file_name"`
	Retention int    `json:"retention"`
}

// Result is printed on stdout as JSON.
type Result struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Error   string `json:"error,omitempty"`

	User      string `json:"user,omitempty"`
	Resources int    `json:"resources"`
	Folders   int    `json:"folders"`

	// Set by check when the server key was pinned in this run. The caller
	// stores both values with the module secrets.
	NewVerifyToken    string `json:"new_verify_token,omitempty"`
	NewVerifyEncToken string `json:"new_verify_enc_token,omitempty"`

	TargetOK bool `json:"target_ok"`

	Entries  int      `json:"entries"`
	Skipped  []string `json:"skipped,omitempty"`
	FileName string   `json:"file_name,omitempty"`
	Size     int64    `json:"size"`
	SHA256   string   `json:"sha256,omitempty"`
	Verified bool     `json:"verified"`
	Uploaded bool     `json:"uploaded"`
	Pruned   []string `json:"pruned,omitempty"`
	Kept     int      `json:"kept"`
	Seconds  int      `json:"seconds"`
}

func main() {
	res := Result{}
	if len(os.Args) != 2 {
		res.Error = "usage: pbexport check|run|seal|pdf < input.json"
		finish(&res, 2)
	}
	res.Command = os.Args[1]
	switch res.Command {
	case "seal":
		runSeal()
	case "pdf":
		runPDF()
	}

	cfg, err := readConfig(os.Stdin)
	if err != nil {
		res.Error = err.Error()
		finish(&res, 2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	start := time.Now()

	if err := useCA(cfg.CAPEM); err != nil {
		res.Error = err.Error()
		finish(&res, 1)
	}
	switch res.Command {
	case "check":
		err = runCheck(ctx, cfg, &res)
	case "run":
		err = runExport(ctx, cfg, &res)
	default:
		err = fmt.Errorf("unknown command %q", res.Command)
	}
	res.Seconds = int(time.Since(start).Seconds())
	if err != nil {
		res.Error = err.Error()
		finish(&res, 1)
	}
	res.OK = true
	finish(&res, 0)
}

func finish(res *Result, code int) {
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		code = 1
	}
	os.Exit(code)
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func readConfig(r io.Reader) (*Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfigSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading the settings: %w", err)
	}
	if len(data) > maxConfigSize {
		return nil, errors.New("the settings document is too large")
	}
	cfg := &Config{}
	dec := json.NewDecoder(bytesReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parsing the settings: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
