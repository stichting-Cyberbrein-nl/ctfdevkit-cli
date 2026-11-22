package ctfnet

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New(srv.URL+"/", "client-1", "test")
	var sleeps []time.Duration
	c.Sleep = func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}
	return c, &sleeps
}

func TestRequestDeviceCodeSendsClientAndScope(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/device/code" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.FormValue("client_id") != "client-1" || r.FormValue("scope") != Scope {
			t.Errorf("form = %v", r.Form)
		}
		io.WriteString(w, `{"device_code":"dev","user_code":"ABCDEFGH","verification_uri":"https://x/oauth/device","verification_uri_complete":"https://x/oauth/device?user_code=ABCDEFGH","expires_in":600,"interval":5}`)
	})
	dc, err := c.RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dc.DeviceCode != "dev" || dc.UserCode != "ABCDEFGH" || dc.Interval != 5 || dc.ExpiresIn != 600 {
		t.Fatalf("dc = %+v", dc)
	}
}

func TestPollTokenHandlesPendingAndSlowDown(t *testing.T) {
	responses := []string{`{"error":"authorization_pending"}`, `{"error":"slow_down"}`}
	c, sleeps := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("grant_type") != deviceGrantType || r.FormValue("device_code") != "dev" || r.FormValue("client_id") != "client-1" {
			t.Errorf("form = %v", r.Form)
		}
		if len(responses) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, responses[0])
			responses = responses[1:]
			return
		}
		io.WriteString(w, `{"token_type":"Bearer","expires_in":3600,"access_token":"acc","refresh_token":"ref"}`)
	})
	tok, err := c.PollToken(context.Background(), DeviceCode{DeviceCode: "dev", ExpiresIn: 600, Interval: 5})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "acc" || tok.RefreshToken != "ref" || tok.ExpiresIn != 3600 {
		t.Fatalf("tok = %+v", tok)
	}
	if want := []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second}; !slices.Equal(*sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", *sleeps, want)
	}
}

func TestPollTokenAccessDenied(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"access_denied"}`)
	})
	if _, err := c.PollToken(context.Background(), DeviceCode{DeviceCode: "dev"}); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied", err)
	}
}

func TestUploadAssignmentSendsArchiveAndParsesData(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/assignments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("headers = %v", r.Header)
		}
		f, fh, err := r.FormFile("archive")
		if err != nil {
			t.Errorf("archive field: %v", err)
		} else {
			body, _ := io.ReadAll(f)
			if string(body) != "zipdata" || fh.Filename != "[12] Test.zip" || fh.Header.Get("Content-Type") != "application/zip" {
				t.Errorf("part = %q %q %q", body, fh.Filename, fh.Header.Get("Content-Type"))
			}
		}
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"data":{"name":"[12] Test","challenge_id":7,"url":"https://x/challenges/7","replaced":false}}`)
	})
	res, err := c.UploadAssignment(context.Background(), "tok", "[12] Test", []byte("zipdata"))
	if err != nil {
		t.Fatal(err)
	}
	if res != (UploadResult{Name: "[12] Test", ChallengeID: 7, URL: "https://x/challenges/7"}) {
		t.Fatalf("res = %+v", res)
	}
}

func TestUploadAssignmentValidationError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"message":"The given data was invalid.","errors":{"archive":["config.json is missing"]}}`)
	})
	_, err := c.UploadAssignment(context.Background(), "tok", "[12] Test", []byte("x"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 || apiErr.Message != "config.json is missing" {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadAssignmentUnauthorized(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"message":"Unauthenticated."}`)
	})
	if _, err := c.UploadAssignment(context.Background(), "tok", "[12] Test", []byte("x")); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}
