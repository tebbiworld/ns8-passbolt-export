//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"bytes"
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	// Only names written by this tool: <prefix>-YYYY-MM-DD_HHMMSS.kdbx. The
	// same pattern decides which files on the target may be pruned, so it must
	// never match anything the user put there.
	fileNameRE = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_-]{0,63})-(\d{4}-\d{2}-\d{2}_\d{6})\.kdbx$`)
	// rclone remote "target:" followed by a relative path without "..".
	remoteRE = regexp.MustCompile(`^target:[A-Za-z0-9 ._/-]{0,200}$`)
)

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func (c *Config) validate() error {
	u, err := url.Parse(c.ServerURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("the Passbolt address must be an https:// URL without user, query or fragment")
	}
	if !strings.Contains(c.PrivateKey, "BEGIN PGP PRIVATE KEY BLOCK") {
		return errors.New("the private key is missing or not an ASCII-armored OpenPGP private key")
	}
	if c.Passphrase == "" {
		return errors.New("the passphrase of the private key is missing")
	}
	if c.CAPEM != "" && !strings.Contains(c.CAPEM, "BEGIN CERTIFICATE") {
		return errors.New("the CA certificate is not in PEM format")
	}
	if (c.VerifyToken == "") != (c.VerifyEncToken == "") {
		return errors.New("incomplete server verification data")
	}
	if c.Remote != "" {
		if !remoteRE.MatchString(c.Remote) || strings.Contains(c.Remote, "..") {
			return errors.New("invalid target path")
		}
	}
	return nil
}

func (c *Config) validateExport() error {
	if len(c.KdbxPassword) < 12 {
		return errors.New("the KeePass password must have at least 12 characters")
	}
	if c.Remote == "" {
		return errors.New("no upload target configured")
	}
	if !fileNameRE.MatchString(c.FileName) {
		return errors.New("invalid file name")
	}
	if c.Retention < 1 || c.Retention > 1000 {
		return errors.New("the retention must be between 1 and 1000")
	}
	return nil
}

// prefixOf returns the name prefix of a file written by this tool.
func prefixOf(name string) (string, bool) {
	m := fileNameRE.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1], true
}
