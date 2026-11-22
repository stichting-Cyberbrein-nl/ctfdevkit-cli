// Package ctfnet is the client for the ctf.cyberbrein.net platform: OAuth2 device
// login (Laravel Passport) and assignment uploads.
package ctfnet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

// Scope is the OAuth scope the DevKit needs to upload assignments.
const Scope = "upload-assignments"

const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

var (
	// ErrAccessDenied means the user denied the device login in the browser.
	ErrAccessDenied = errors.New("access denied")
	// ErrExpired means the device code expired before the user approved it.
	ErrExpired = errors.New("device code expired")
	// ErrUnauthorized means the access token was rejected (HTTP 401).
	ErrUnauthorized = errors.New("unauthorized")
)

// APIError is a non-success platform response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("platform returned HTTP %d", e.Status)
	}
	return fmt.Sprintf("platform returned HTTP %d: %s", e.Status, e.Message)
}

// Client talks to the platform.
type Client struct {
	BaseURL   string
	ClientID  string
	UserAgent string
	HTTP      *http.Client
	Sleep     func(context.Context, time.Duration) error
}

// New returns a client for the platform at baseURL.
func New(baseURL, clientID, version string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		ClientID:  clientID,
		UserAgent: "devkit-cli/" + version,
		HTTP:      &http.Client{Timeout: 2 * time.Minute},
		Sleep:     sleep,
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// DeviceCode is the response of the device authorization request.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// Token is an OAuth access/refresh token pair.
type Token struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// User is the logged-in platform account.
type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// UploadResult describes the challenge an upload created or replaced.
type UploadResult struct {
	Name        string `json:"name"`
	ChallengeID int64  `json:"challenge_id"`
	URL         string `json:"url"`
	Replaced    bool   `json:"replaced"`
}

// RequestDeviceCode starts a device login.
func (c *Client) RequestDeviceCode(ctx context.Context) (DeviceCode, error) {
	var dc DeviceCode
	status, body, err := c.postForm(ctx, "/oauth/device/code", url.Values{"client_id": {c.ClientID}, "scope": {Scope}})
	if err != nil {
		return dc, fmt.Errorf("requesting device code: %w", err)
	}
	if status != http.StatusOK {
		return dc, oauthError(status, body)
	}
	if err := json.Unmarshal(body, &dc); err != nil {
		return dc, fmt.Errorf("parsing device code: %w", err)
	}
	return dc, nil
}

// PollToken waits until the user approves or denies the device login.
func (c *Client) PollToken(ctx context.Context, dc DeviceCode) (Token, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expires := time.Duration(dc.ExpiresIn) * time.Second
	var waited time.Duration
	form := url.Values{"grant_type": {deviceGrantType}, "client_id": {c.ClientID}, "device_code": {dc.DeviceCode}}

	for {
		waited += interval
		if expires > 0 && waited > expires {
			return Token{}, ErrExpired
		}
		if err := c.Sleep(ctx, interval); err != nil {
			return Token{}, err
		}

		status, body, err := c.postForm(ctx, "/oauth/token", form)
		if err != nil {
			return Token{}, fmt.Errorf("polling for token: %w", err)
		}
		if status == http.StatusOK {
			return parseToken(body)
		}
		var e oauthErr
		_ = json.Unmarshal(body, &e)
		switch e.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "access_denied":
			return Token{}, ErrAccessDenied
		case "expired_token":
			return Token{}, ErrExpired
		default:
			return Token{}, oauthError(status, body)
		}
	}
}

// Refresh exchanges a refresh token for a new token pair (the refresh token rotates).
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {c.ClientID}, "scope": {Scope}}
	status, body, err := c.postForm(ctx, "/oauth/token", form)
	if err != nil {
		return Token{}, fmt.Errorf("refreshing token: %w", err)
	}
	if status != http.StatusOK {
		return Token{}, oauthError(status, body)
	}
	return parseToken(body)
}

// CurrentUser returns the account the access token belongs to.
func (c *Client) CurrentUser(ctx context.Context, accessToken string) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/user", nil)
	if err != nil {
		return User{}, fmt.Errorf("building user request: %w", err)
	}
	c.authorize(req, accessToken)
	status, body, err := c.do(req)
	if err != nil {
		return User{}, fmt.Errorf("fetching user: %w", err)
	}
	if status == http.StatusUnauthorized {
		return User{}, ErrUnauthorized
	}
	if status != http.StatusOK {
		return User{}, apiError(status, body)
	}
	var resp struct {
		Data User `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return User{}, fmt.Errorf("parsing user: %w", err)
	}
	return resp.Data, nil
}

// UploadAssignment uploads a zipped assignment folder.
func (c *Client) UploadAssignment(ctx context.Context, accessToken, name string, archive []byte) (UploadResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="archive"; filename=%q`, name+".zip"))
	h.Set("Content-Type", "application/zip")
	part, err := mw.CreatePart(h)
	if err != nil {
		return UploadResult{}, fmt.Errorf("building upload: %w", err)
	}
	if _, err := part.Write(archive); err != nil {
		return UploadResult{}, fmt.Errorf("building upload: %w", err)
	}
	if err := mw.Close(); err != nil {
		return UploadResult{}, fmt.Errorf("building upload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/assignments", &buf)
	if err != nil {
		return UploadResult{}, fmt.Errorf("building upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.authorize(req, accessToken)
	status, body, err := c.do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("uploading assignment: %w", err)
	}
	switch status {
	case http.StatusOK, http.StatusCreated:
		var resp struct {
			Data UploadResult `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return UploadResult{}, fmt.Errorf("parsing upload result: %w", err)
		}
		return resp.Data, nil
	case http.StatusUnauthorized:
		return UploadResult{}, ErrUnauthorized
	default:
		return UploadResult{}, apiError(status, body)
	}
}

func (c *Client) authorize(req *http.Request, accessToken string) {
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return c.do(req)
}

func (c *Client) do(req *http.Request) (int, []byte, error) {
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("reading response: %w", err)
	}
	return resp.StatusCode, body, nil
}

type oauthErr struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
	Message     string `json:"message"`
}

func oauthError(status int, body []byte) error {
	var e oauthErr
	_ = json.Unmarshal(body, &e)
	msg := e.Description
	if msg == "" {
		msg = e.Error
	}
	if msg == "" {
		msg = e.Message
	}
	return &APIError{Status: status, Message: msg}
}

func apiError(status int, body []byte) error {
	var e struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
	}
	_ = json.Unmarshal(body, &e)
	msg := e.Message
	if a := e.Errors["archive"]; len(a) > 0 {
		msg = a[0]
	}
	return &APIError{Status: status, Message: msg}
}

func parseToken(body []byte) (Token, error) {
	var t Token
	if err := json.Unmarshal(body, &t); err != nil {
		return t, fmt.Errorf("parsing token: %w", err)
	}
	if t.AccessToken == "" {
		return t, errors.New("platform returned no access token")
	}
	return t, nil
}
