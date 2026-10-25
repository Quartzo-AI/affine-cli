// Package authsession signs the CLI in to an AFFiNE >= 0.27 server.
//
// Since 0.27 the server accepts a Bearer only when it is a JWT minted by the
// session endpoints (/api/auth/sign-in -> /session/exchange -> /session/refresh);
// any other Bearer (including the retired personal access tokens "ut_...") is
// ignored and the request is treated as anonymous. This package performs that
// handshake and caches the rotating refresh token in a 0600 file, so a command
// refreshes the session instead of opening a new device session on every run.
package authsession

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultClientVersion is sent in x-affine-version when /info is unreachable.
	DefaultClientVersion = "0.27.4"

	clientKindHeader  = "x-affine-client-kind"
	versionHeader     = "x-affine-version"
	expirySkew        = 60 * time.Second
	lockWait          = 15 * time.Second
	lockStale         = 45 * time.Second
	requestTimeout    = 20 * time.Second
	sessionPlatform   = "electron"
	sessionDeviceName = "affine-cli"
)

// ErrNoCredentials means there is neither a cached session nor email/password.
var ErrNoCredentials = errors.New("no AFFiNE session: set AFFINE_EMAIL and AFFINE_PASSWORD (agent account, OpenBao /agents/showrunner/affine)")

// AuthError is a server rejection of the sign-in or of the refresh token.
type AuthError struct {
	Step   string
	Status int
	Detail string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("AFFiNE %s rejected (HTTP %d): %s", e.Step, e.Status, e.Detail)
}

// Options describes where to sign in and where the session is cached.
type Options struct {
	BaseURL  string
	Email    string
	Password string
	// CachePath is the session file; empty disables caching.
	CachePath string
	HTTP      *http.Client
	Now       func() time.Time
}

// Session is the cached credential. Never printed or logged.
type Session struct {
	BaseURL          string    `json:"base_url"`
	Email            string    `json:"email,omitempty"`
	InstallationID   string    `json:"installation_id"`
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

func (s *Session) accessValid(now time.Time) bool {
	return s != nil && s.AccessToken != "" && now.Add(expirySkew).Before(s.AccessExpiresAt)
}

func (s *Session) refreshValid(now time.Time) bool {
	return s != nil && s.RefreshToken != "" && now.Before(s.RefreshExpiresAt)
}

// HasCachedSession reports whether a usable-looking session file exists for baseURL.
func HasCachedSession(cachePath, baseURL string) bool {
	s := readCache(cachePath)
	return s != nil && sameServer(s.BaseURL, baseURL) && s.RefreshToken != ""
}

// IsJWT reports whether token has the shape the server accepts as a Bearer
// (three dot-separated parts, no "aff_mcp_v1." prefix), mirroring isLikelyJwt.
func IsJWT(token string) bool {
	if strings.HasPrefix(token, "aff_mcp_v1.") {
		return false
	}
	return len(strings.Split(token, ".")) == 3
}

// AccessToken returns a valid JWT, refreshing or signing in as needed.
func AccessToken(ctx context.Context, o Options) (string, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: requestTimeout}
	}
	o.BaseURL = strings.TrimRight(o.BaseURL, "/")

	if s := readCache(o.CachePath); matches(s, o) && s.accessValid(o.Now()) {
		return s.AccessToken, nil
	}
	unlock, err := lock(o.CachePath)
	if err != nil {
		return "", err
	}
	defer unlock()

	// Another process may have refreshed while this one waited for the lock.
	s := readCache(o.CachePath)
	if !matches(s, o) {
		keep := ""
		if s != nil {
			keep = s.InstallationID
		}
		s = &Session{BaseURL: o.BaseURL, Email: o.Email, InstallationID: keep}
	}
	if s.accessValid(o.Now()) {
		return s.AccessToken, nil
	}
	if s.InstallationID == "" {
		s.InstallationID = newUUID()
	}

	version := serverVersion(ctx, o)
	if s.refreshValid(o.Now()) {
		err := refresh(ctx, o, s, version)
		if err == nil {
			writeCache(o.CachePath, s)
			return s.AccessToken, nil
		}
		var ae *AuthError
		if !errors.As(err, &ae) || o.Password == "" {
			return "", err
		}
	}
	if o.Email == "" || o.Password == "" {
		return "", ErrNoCredentials
	}
	if err := login(ctx, o, s, version); err != nil {
		return "", err
	}
	writeCache(o.CachePath, s)
	return s.AccessToken, nil
}

// Clear removes the cached session file.
func Clear(cachePath string) error {
	if cachePath == "" {
		return nil
	}
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func matches(s *Session, o Options) bool {
	if s == nil || !sameServer(s.BaseURL, o.BaseURL) {
		return false
	}
	return o.Email == "" || strings.EqualFold(s.Email, o.Email)
}

func sameServer(a, b string) bool {
	return strings.TrimRight(a, "/") == strings.TrimRight(b, "/")
}

type tokenPair struct {
	AccessToken      string    `json:"accessToken"`
	ExpiresIn        int64     `json:"expiresIn"`
	RefreshToken     string    `json:"refreshToken"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
}

func (p tokenPair) apply(s *Session, now time.Time) error {
	if p.AccessToken == "" || p.RefreshToken == "" {
		return errors.New("AFFiNE session response carried no tokens")
	}
	s.AccessToken = p.AccessToken
	s.AccessExpiresAt = now.Add(time.Duration(p.ExpiresIn) * time.Second)
	s.RefreshToken = p.RefreshToken
	s.RefreshExpiresAt = p.RefreshExpiresAt
	return nil
}

func login(ctx context.Context, o Options, s *Session, version string) error {
	var signed struct {
		ExchangeCode string `json:"exchangeCode"`
	}
	if err := post(ctx, o, "sign-in", "/api/auth/sign-in", version,
		map[string]string{"email": o.Email, "password": o.Password}, &signed); err != nil {
		return err
	}
	if signed.ExchangeCode == "" {
		return errors.New("AFFiNE sign-in returned no exchangeCode (server too old for JWT sessions?)")
	}
	var pair tokenPair
	if err := post(ctx, o, "session exchange", "/api/auth/session/exchange", version, map[string]string{
		"code":           signed.ExchangeCode,
		"installationId": s.InstallationID,
		"platform":       sessionPlatform,
		"deviceName":     sessionDeviceName,
	}, &pair); err != nil {
		return err
	}
	s.Email = o.Email
	return pair.apply(s, o.Now())
}

func refresh(ctx context.Context, o Options, s *Session, version string) error {
	var pair tokenPair
	if err := post(ctx, o, "session refresh", "/api/auth/session/refresh", version,
		map[string]string{"refreshToken": s.RefreshToken}, &pair); err != nil {
		return err
	}
	return pair.apply(s, o.Now())
}

func post(ctx context.Context, o Options, step, path, version string, body any, out any) error {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(clientKindHeader, "native")
	req.Header.Set(versionHeader, version)
	req.Header.Set("User-Agent", "affine-pp-cli")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("AFFiNE %s: %w", step, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return &AuthError{Step: step, Status: resp.StatusCode, Detail: errorDetail(data)}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("AFFiNE %s: unreadable response: %w", step, err)
	}
	return nil
}

// errorDetail keeps only the server's error name/message; bodies never echo secrets.
func errorDetail(data []byte) string {
	var e struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(data, &e) == nil {
		switch {
		case e.Name != "" && e.Message != "":
			return e.Name + ": " + e.Message
		case e.Message != "":
			return e.Message
		case e.Code != "":
			return e.Code
		}
	}
	return "no detail"
}

// serverVersion reads the server's advertised compatible client version.
func serverVersion(ctx context.Context, o Options) string {
	if v := os.Getenv("AFFINE_CLIENT_VERSION"); v != "" {
		return v
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/info", nil)
	if err != nil {
		return DefaultClientVersion
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return DefaultClientVersion
	}
	defer func() { _ = resp.Body.Close() }()
	var info struct {
		Compatibility string `json:"compatibility"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&info) != nil || info.Compatibility == "" {
		return DefaultClientVersion
	}
	return info.Compatibility
}

func readCache(path string) *Session {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s Session
	if json.Unmarshal(data, &s) != nil {
		return nil
	}
	return &s
}

// writeCache is best effort: a read-only home must not fail the command.
func writeCache(path string, s *Session) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, _ := json.Marshal(s)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// lock serializes refresh-token rotation across concurrent CLI processes.
func lock(cachePath string) (func(), error) {
	if cachePath == "" {
		return func() {}, nil
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		return func() {}, nil
	}
	path := cachePath + ".lock"
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > lockStale {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("AFFiNE session lock %s busy for %s", path, lockWait)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
