package cli

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/browser"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ctfnet"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

var errNotLoggedIn = errors.New("not logged in — run devkit login")

// platformClient returns a client for the configured ctf.cyberbrein.net platform.
func platformClient(ctx context.Context) *ctfnet.Client {
	cfg := configFrom(ctx)
	return ctfnet.New(cfg.PlatformURL, cfg.PlatformClientID, Version)
}

// deviceLogin runs the OAuth device login and stores the resulting credentials.
func deviceLogin(ctx context.Context, c *ctfnet.Client) (ctfnet.Credentials, error) {
	dc, err := c.RequestDeviceCode(ctx)
	if err != nil {
		return ctfnet.Credentials{}, err
	}
	output.Info("Open " + dc.VerificationURI + " and enter the code " + formatUserCode(dc.UserCode))
	_ = browser.Open(dc.VerificationURIComplete, platformFrom(ctx))
	output.Info("Waiting for approval in the browser...")

	tok, err := c.PollToken(ctx, dc)
	switch {
	case errors.Is(err, ctfnet.ErrAccessDenied):
		return ctfnet.Credentials{}, errors.New("login was denied in the browser")
	case errors.Is(err, ctfnet.ErrExpired):
		return ctfnet.Credentials{}, errors.New("the login code expired; run devkit login again")
	case err != nil:
		return ctfnet.Credentials{}, err
	}

	user, err := c.CurrentUser(ctx, tok.AccessToken)
	if err != nil {
		return ctfnet.Credentials{}, err
	}
	creds := ctfnet.Credentials{
		PlatformURL:  c.BaseURL,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
		User:         user,
	}
	if err := ctfnet.SaveCredentials(creds); err != nil {
		return ctfnet.Credentials{}, err
	}
	return creds, nil
}

// formatUserCode shows an 8-letter code as XXXX-XXXX.
func formatUserCode(code string) string {
	if len(code) == 8 && !strings.Contains(code, "-") {
		return code[:4] + "-" + code[4:]
	}
	return code
}

// ensureLoggedIn returns valid credentials for c's platform, refreshing or logging in as needed.
func ensureLoggedIn(ctx context.Context, c *ctfnet.Client) (ctfnet.Credentials, error) {
	creds, err := ctfnet.LoadCredentials()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ctfnet.Credentials{}, err
	}
	if err != nil || creds.PlatformURL != c.BaseURL {
		return loginOrFail(ctx, c)
	}
	if time.Until(creds.ExpiresAt) > 60*time.Second {
		return creds, nil
	}
	refreshed, err := refreshCredentials(ctx, c, creds)
	if err != nil {
		_ = ctfnet.DeleteCredentials()
		return loginOrFail(ctx, c)
	}
	return refreshed, nil
}

// refreshCredentials exchanges the refresh token and stores the rotated tokens.
func refreshCredentials(ctx context.Context, c *ctfnet.Client, creds ctfnet.Credentials) (ctfnet.Credentials, error) {
	tok, err := c.Refresh(ctx, creds.RefreshToken)
	if err != nil {
		return ctfnet.Credentials{}, err
	}
	creds.AccessToken = tok.AccessToken
	creds.RefreshToken = tok.RefreshToken
	creds.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if err := ctfnet.SaveCredentials(creds); err != nil {
		return ctfnet.Credentials{}, err
	}
	return creds, nil
}

func loginOrFail(ctx context.Context, c *ctfnet.Client) (ctfnet.Credentials, error) {
	if !isInteractive() {
		return ctfnet.Credentials{}, errNotLoggedIn
	}
	return deviceLogin(ctx, c)
}
