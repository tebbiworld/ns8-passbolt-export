//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// sealIterations matches the WebCrypto PBKDF2 call of the emergency sheet page.
const sealIterations = 600000

type sealRequest struct {
	Password  string `json:"password"`
	Plaintext string `json:"plaintext"`
}

type sealed struct {
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	KDF        string `json:"kdf,omitempty"`
	Iterations int    `json:"iterations,omitempty"`
	Salt       string `json:"salt,omitempty"`
	Nonce      string `json:"nonce,omitempty"`
	Ciphertext string `json:"ciphertext,omitempty"`
}

// seal encrypts a document for the browser with a key derived from the
// administrator's password (PBKDF2-SHA256 + AES-256-GCM). The NS8 agent keeps
// task output in Redis for hours: it must only ever see the ciphertext.
func seal(r io.Reader) sealed {
	var req sealRequest
	data, err := io.ReadAll(io.LimitReader(r, maxConfigSize+1))
	if err == nil && len(data) > maxConfigSize {
		err = errors.New("the request is too large")
	}
	if err == nil {
		err = json.Unmarshal(data, &req)
	}
	if err == nil && (len(req.Password) < 1 || req.Plaintext == "") {
		err = errors.New("password and plaintext are required")
	}
	if err != nil {
		return sealed{Error: err.Error()}
	}
	salt := make([]byte, 16)
	nonce := make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return sealed{Error: err.Error()}
	}
	if _, err := rand.Read(nonce); err != nil {
		return sealed{Error: err.Error()}
	}
	key, err := pbkdf2.Key(sha256.New, req.Password, salt, sealIterations, 32)
	if err != nil {
		return sealed{Error: err.Error()}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return sealed{Error: err.Error()}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return sealed{Error: err.Error()}
	}
	ct := gcm.Seal(nil, nonce, []byte(req.Plaintext), nil)
	b64 := base64.StdEncoding.EncodeToString
	return sealed{OK: true, KDF: "PBKDF2-SHA256", Iterations: sealIterations, Salt: b64(salt), Nonce: b64(nonce), Ciphertext: b64(ct)}
}

func runSeal() {
	out := seal(os.Stdin)
	_ = json.NewEncoder(os.Stdout).Encode(out)
	if !out.OK {
		os.Exit(1)
	}
	os.Exit(0)
}
