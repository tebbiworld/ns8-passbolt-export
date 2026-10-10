//
// Copyright (C) 2026 tebbi
// SPDX-License-Identifier: GPL-3.0-or-later
//

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

func httpClient(caPEM string) (*http.Client, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if caPEM != "" && !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, errors.New("the CA certificate could not be read")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
	transport.Proxy = nil
	return &http.Client{Transport: transport, Timeout: 2 * time.Minute}, nil
}

// login returns a logged-in client. On the first contact (no stored
// verification token) the server key is pinned and the new token pair is
// returned through res; afterwards every login checks the server against it,
// so a replaced server or key is refused instead of trusted.
func login(ctx context.Context, cfg *Config, res *Result) (*api.Client, error) {
	hc, err := httpClient(cfg.CAPEM)
	if err != nil {
		return nil, err
	}
	client, err := api.NewClient(hc, "ns8-passbolt-export", cfg.ServerURL, cfg.PrivateKey, cfg.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("reading the private key: %w", err)
	}

	if cfg.VerifyToken != "" {
		if err := client.VerifyServer(ctx, cfg.VerifyToken, cfg.VerifyEncToken); err != nil {
			return nil, fmt.Errorf("the server identity does not match the one pinned at setup (server key changed?): %w", err)
		}
	} else {
		token, encToken, err := client.SetupServerVerification(ctx)
		if err != nil {
			return nil, fmt.Errorf("pinning the server key: %w", err)
		}
		res.NewVerifyToken, res.NewVerifyEncToken = token, encToken
	}

	if cfg.TOTPSecret != "" {
		helper.AddMFACallbackTOTP(client, 3, 10*time.Second, 0, cfg.TOTPSecret)
	}
	if err := client.Login(ctx); err != nil {
		if strings.Contains(err.Error(), "MFA callback is not defined") {
			return nil, errors.New("logging in: the account requires 2FA: store its TOTP secret in the settings")
		}
		return nil, explain("logging in", err)
	}
	return client, nil
}

func explain(op string, err error) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusForbidden:
			return fmt.Errorf("%s: access denied (2FA required, wrong TOTP secret or missing permission): %w", op, err)
		case http.StatusUnauthorized:
			return fmt.Errorf("%s: not authenticated (key, passphrase or account wrong): %w", op, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func logout(client *api.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Logout(ctx); err != nil {
		logf("logout failed: %v", err)
	}
}

func runCheck(ctx context.Context, cfg *Config, res *Result) error {
	client, err := login(ctx, cfg, res)
	if err != nil {
		return err
	}
	defer logout(client)

	if me, err := client.GetMe(ctx); err == nil {
		res.User = me.Username
	}
	folders, err := client.GetFolders(ctx, nil)
	if err != nil {
		return explain("listing folders", err)
	}
	resources, err := client.GetResources(ctx, nil)
	if err != nil {
		return explain("listing resources", err)
	}
	res.Folders, res.Resources = len(folders), len(resources)
	logf("logged in, %d resources in %d folders visible", res.Resources, res.Folders)

	if cfg.Remote != "" {
		if err := probeTarget(ctx, cfg.Remote); err != nil {
			return err
		}
		res.TargetOK = true
	}
	return nil
}
