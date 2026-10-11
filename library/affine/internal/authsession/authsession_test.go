package authsession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type fakeServer struct {
	signIns, exchanges, refreshes int32
	rejectRefresh                 bool
	refreshExpiresIn              time.Duration
}

func (f *fakeServer) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"compatibility":"0.27.4"}`))
	})
	need := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("x-affine-client-kind") != "native" || r.Header.Get("x-affine-version") != "0.27.4" {
			t.Errorf("missing native client headers on %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return false
		}
		return true
	}
	pair := func(w http.ResponseWriter, n int32) {
		exp := f.refreshExpiresIn
		if exp == 0 {
			exp = 24 * time.Hour
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tokenType": "Bearer", "accessToken": "a.b.jwt" + string(rune('0'+n)), "expiresIn": 900,
			"refreshToken": "refresh" + string(rune('0'+n)), "refreshExpiresAt": time.Now().Add(exp),
		})
	}
	mux.HandleFunc("/api/auth/sign-in", func(w http.ResponseWriter, r *http.Request) {
		if !need(w, r) {
			return
		}
		var b map[string]string
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b["password"] != "secret" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"name":"WRONG_USER_PASSWORD","message":"Wrong user email or password"}`))
			return
		}
		atomic.AddInt32(&f.signIns, 1)
		_, _ = w.Write([]byte(`{"id":"u1","exchangeCode":"code"}`))
	})
	mux.HandleFunc("/api/auth/session/exchange", func(w http.ResponseWriter, r *http.Request) {
		if !need(w, r) {
			return
		}
		var b map[string]string
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b["code"] != "code" || len(b["installationId"]) != 36 || b["platform"] != "electron" {
			t.Errorf("bad exchange body %v", b)
		}
		pair(w, atomic.AddInt32(&f.exchanges, 1))
	})
	mux.HandleFunc("/api/auth/session/refresh", func(w http.ResponseWriter, r *http.Request) {
		if !need(w, r) {
			return
		}
		if f.rejectRefresh {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"invalid_refresh_token"}`))
			return
		}
		pair(w, 5+atomic.AddInt32(&f.refreshes, 1))
	})
	return mux
}

func opts(srv *httptest.Server, dir string, now func() time.Time) Options {
	return Options{BaseURL: srv.URL, Email: "a@b.c", Password: "secret", CachePath: filepath.Join(dir, "session.json"), Now: now}
}

func TestLoginCachesAndReuses(t *testing.T) {
	f := &fakeServer{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	o := opts(srv, t.TempDir(), nil)

	tok, err := AccessToken(context.Background(), o)
	if err != nil || !IsJWT(tok) {
		t.Fatalf("login: %q %v", tok, err)
	}
	if tok2, _ := AccessToken(context.Background(), o); tok2 != tok || f.signIns != 1 {
		t.Fatalf("second call must reuse the cache (signIns=%d)", f.signIns)
	}
	st, err := os.Stat(o.CachePath)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("cache must be 0600: %v %v", st, err)
	}
	if !HasCachedSession(o.CachePath, srv.URL) {
		t.Fatal("HasCachedSession should see the file")
	}
}

func TestRefreshWhenAccessExpired(t *testing.T) {
	f := &fakeServer{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	o := opts(srv, t.TempDir(), nil)
	if _, err := AccessToken(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(20 * time.Minute)
	o.Now = func() time.Time { return later }
	o.Password = "" // refresh must not need the password
	tok, err := AccessToken(context.Background(), o)
	if err != nil || tok != "a.b.jwt6" || f.refreshes != 1 || f.signIns != 1 {
		t.Fatalf("refresh: tok=%q err=%v refreshes=%d signIns=%d", tok, err, f.refreshes, f.signIns)
	}
}

func TestRejectedRefreshFallsBackToPasswordLogin(t *testing.T) {
	f := &fakeServer{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	o := opts(srv, t.TempDir(), nil)
	if _, err := AccessToken(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	f.rejectRefresh = true
	o.Now = func() time.Time { return time.Now().Add(20 * time.Minute) }
	if _, err := AccessToken(context.Background(), o); err != nil || f.signIns != 2 {
		t.Fatalf("expected re-login, err=%v signIns=%d", err, f.signIns)
	}
	// installation id survives re-login so the server sees one device
	s := readCache(o.CachePath)
	if s == nil || s.InstallationID == "" {
		t.Fatal("installation id lost")
	}
}

func TestWrongPasswordIsAuthError(t *testing.T) {
	f := &fakeServer{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	o := opts(srv, t.TempDir(), nil)
	o.Password = "nope"
	_, err := AccessToken(context.Background(), o)
	var ae *AuthError
	if !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("want AuthError 400, got %v", err)
	}
	if got := err.Error(); containsAny(got, "nope", "secret") {
		t.Fatalf("error leaks a secret: %s", got)
	}
}

func TestNoCredentialsNoCache(t *testing.T) {
	f := &fakeServer{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	_, err := AccessToken(context.Background(), Options{BaseURL: srv.URL, CachePath: filepath.Join(t.TempDir(), "s.json")})
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("want ErrNoCredentials, got %v", err)
	}
}

func TestIsJWT(t *testing.T) {
	for tok, want := range map[string]bool{"a.b.c": true, "ut_abcdef": false, "aff_mcp_v1.x.y": false, "a.b": false, "": false} {
		if IsJWT(tok) != want {
			t.Errorf("IsJWT(%q) != %v", tok, want)
		}
	}
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		for i := 0; i+len(x) <= len(s); i++ {
			if s[i:i+len(x)] == x {
				return true
			}
		}
	}
	return false
}
