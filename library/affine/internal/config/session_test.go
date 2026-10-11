package config

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func loadWith(t *testing.T, base string, env map[string]string) *Config {
	t.Helper()
	for _, k := range []string{"AFFINE_TOKEN", "AFFINE_EMAIL", "AFFINE_USERNAME", "AFFINE_PASSWORD", "AFFINE_CLIENT_VERSION"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	t.Setenv("AFFINE_BASE_URL", base)
	cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLegacyTokenStaysVisibleButNoSession(t *testing.T) {
	cfg := loadWith(t, "http://127.0.0.1:1", map[string]string{"AFFINE_TOKEN": "ut_legacy"})
	if cfg.SessionConfigured() {
		t.Fatal("no session without email/password")
	}
	if got := cfg.AuthHeader(); got != "Bearer ut_legacy" {
		t.Fatalf("legacy token must stay visible so doctor can reject it, got %q", got)
	}
}

func TestJWTTokenUsedAsIs(t *testing.T) {
	cfg := loadWith(t, "http://127.0.0.1:1", map[string]string{"AFFINE_TOKEN": "a.b.c"})
	if cfg.AuthHeader() != "Bearer a.b.c" || cfg.SessionError() != nil {
		t.Fatal("a JWT in AFFINE_TOKEN is used directly")
	}
}

func TestSessionWinsOverLegacyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/sign-in":
			_, _ = w.Write([]byte(`{"exchangeCode":"c"}`))
		case "/api/auth/session/exchange":
			_, _ = w.Write([]byte(`{"accessToken":"h.p.s","expiresIn":900,"refreshToken":"r","refreshExpiresAt":"2099-01-01T00:00:00Z"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	cfg := loadWith(t, srv.URL, map[string]string{"AFFINE_TOKEN": "ut_legacy", "AFFINE_EMAIL": "a@b.c", "AFFINE_PASSWORD": "pw"})
	h := cfg.AuthHeader()
	if h != "Bearer h.p.s" || cfg.AuthSource != "session" {
		t.Fatalf("session must win, got %q source=%q", h, cfg.AuthSource)
	}
	if strings.Contains(cfg.AuthSource, "pw") {
		t.Fatal("source leaks")
	}
}
