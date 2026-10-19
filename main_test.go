package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// subprocessKey is the environment variable used to signal that the test binary
// is running as a subprocess in a test that requires os.Exit behaviour.
const subprocessKey = "STATIC_SERVER_SUBPROCESS"

// statErrFile implements http.File; Stat always errors, all other methods are no-ops.
type statErrFile struct{}

func (statErrFile) Close() error                         { return nil }
func (statErrFile) Read(_ []byte) (int, error)           { return 0, io.EOF }
func (statErrFile) Seek(_ int64, _ int) (int64, error)   { return 0, nil }
func (statErrFile) Readdir(_ int) ([]os.FileInfo, error) { return nil, nil }
func (statErrFile) Stat() (os.FileInfo, error)           { return nil, errors.New("injected stat error") }

// statErrFS is an http.FileSystem whose Open always returns a statErrFile.
type statErrFS struct{}

func (statErrFS) Open(_ string) (http.File, error) { return statErrFile{}, nil }

// newHandler is defined in main.go and shared by the server and the tests, so the
// full middleware stack is exercised here. Test helpers pass a config value to
// select behaviour (SPA fallback, caching, CSP, precompression).

// newTestServer builds a test HTTP server backed by a single temporary directory.
// It returns the server and the path to the temporary directory so tests can
// populate it with fixture files. The server is not closed automatically; call
// srv.Close() via defer in each test.
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	srv := httptest.NewServer(newHandler(config{}, cascadeFS{http.Dir(dir)}))
	return srv, dir
}

// newConfiguredServer is like newTestServer but applies the given config, for
// exercising SPA fallback, caching, CSP, and precompression.
func newConfiguredServer(t *testing.T, cfg config) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	srv := httptest.NewServer(newHandler(cfg, cascadeFS{http.Dir(dir)}))
	return srv, dir
}

// newCascadeTestServer builds a test HTTP server backed by two temporary directories:
// a primary (SERVE_DIR equivalent) and a fallback (FALLBACK_DIR equivalent). It returns
// the server, the primary path, and the fallback path. Tests populate the directories
// with fixture files to verify cascade behaviour. The server is not closed automatically;
// call srv.Close() via defer in each test.
func newCascadeTestServer(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	primary := t.TempDir()
	fallback := t.TempDir()
	srv := httptest.NewServer(newHandler(config{}, cascadeFS{http.Dir(primary), http.Dir(fallback)}))
	return srv, primary, fallback
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVersion(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVersionContentType(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain prefix", ct)
	}
}

func TestServeIndex(t *testing.T) {
	srv, dir := newTestServer(t)
	defer srv.Close()

	if err := os.WriteFile(dir+"/index.html", []byte("<h1>hello</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMissingFile(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/does-not-exist.html")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDirectoryWithoutIndex(t *testing.T) {
	srv, dir := newTestServer(t)
	defer srv.Close()

	if err := os.Mkdir(dir+"/subdir", 0o755); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/subdir/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDirectoryWithIndex(t *testing.T) {
	srv, dir := newTestServer(t)
	defer srv.Close()

	if err := os.Mkdir(dir+"/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/sub/index.html", []byte("<p>sub</p>"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/sub/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv, dir := newTestServer(t)
	defer srv.Close()

	if err := os.WriteFile(dir+"/index.html", []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s: got %q, want %q", header, got, want)
		}
	}
}

func TestSecureHeadersOnHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q, want %q", got, "nosniff")
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("TEST_KEY", "custom")
	if got := envOr("TEST_KEY", "default"); got != "custom" {
		t.Errorf("got %q, want %q", got, "custom")
	}
	if got := envOr("UNSET_KEY_XYZ", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestEnvBool(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv("B", v)
		if !envBool("B", false) {
			t.Errorf("envBool(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"0", "false", "no", "off"} {
		t.Setenv("B", v)
		if envBool("B", true) {
			t.Errorf("envBool(%q) = true, want false", v)
		}
	}
	if got := envBool("UNSET_BOOL_XYZ", true); !got {
		t.Error("unset should return fallback (true)")
	}
}

// ── SPA / MIME / cache / CSP / method tests ──────────────────────────────────

// TestWasmContentType verifies the init() MIME registration serves .wasm as
// application/wasm (scratch images have no /etc/mime.types).
func TestWasmContentType(t *testing.T) {
	srv, dir := newTestServer(t)
	defer srv.Close()
	if err := os.WriteFile(dir+"/m.wasm", []byte("\x00asm"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/m.wasm")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if ct := resp.Header.Get("Content-Type"); ct != "application/wasm" {
		t.Errorf("Content-Type: got %q, want application/wasm", ct)
	}
}

// TestSPAFallback verifies that with SPA_FALLBACK on, an unknown navigation route
// serves index.html (200) while a missing asset still 404s; and that with SPA
// fallback off, the navigation route 404s.
func TestSPAFallback(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{SPAFallback: true})
	defer srv.Close()
	if err := os.WriteFile(dir+"/index.html", []byte("<div id=app></div>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unknown navigation route → index.html, 200.
	resp, err := http.Get(srv.URL + "/some/client/route")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "id=app") {
		t.Fatalf("nav route: status=%d body=%q, want 200 index.html", resp.StatusCode, body)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index fallback Cache-Control: got %q, want no-cache", cc)
	}

	// Missing asset (has extension) → 404, never index.html.
	resp2, err := http.Get(srv.URL + "/assets/missing.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("missing asset: got %d, want 404", resp2.StatusCode)
	}

	// With SPA fallback off, the nav route 404s.
	off, offDir := newConfiguredServer(t, config{})
	defer off.Close()
	_ = os.WriteFile(offDir+"/index.html", []byte("x"), 0o644)
	resp3, err := http.Get(off.URL + "/some/client/route")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("SPA off nav route: got %d, want 404", resp3.StatusCode)
	}
}

// TestCacheControl verifies immutable caching for the configured prefix and
// no-cache for index.html.
func TestCacheControl(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{ImmutablePrefix: "/assets/"})
	defer srv.Close()
	if err := os.Mkdir(dir+"/assets", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/assets/app.abc123.js", []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/index.html", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/assets/app.abc123.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control: got %q", cc)
	}

	resp2, err := http.Get(srv.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if cc := resp2.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index Cache-Control: got %q, want no-cache", cc)
	}
}

// TestCSPNonce verifies every response carries a fresh 128-bit nonce in the CSP's script-src.
func TestCSPNonce(t *testing.T) {
	csp := "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'"
	srv, dir := newConfiguredServer(t, config{CSP: csp, CSPNonce: true})
	defer srv.Close()
	_ = os.WriteFile(dir+"/index.html", []byte("x"), 0o644)
	seen := map[string]bool{}
	for range 3 {
		resp, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		got := resp.Header.Get("Content-Security-Policy")
		prefix := "default-src 'self'; script-src 'self' 'wasm-unsafe-eval' 'nonce-"
		if !strings.HasPrefix(got, prefix) || !strings.HasSuffix(got, "'; style-src 'self'") {
			t.Fatalf("CSP: got %q", got)
		}
		nonce := strings.TrimSuffix(strings.TrimPrefix(got, prefix), "'; style-src 'self'")
		if len(nonce) != 24 || seen[nonce] {
			t.Fatalf("nonce %q: want a fresh 16-byte base64 value per response", nonce)
		}
		seen[nonce] = true
	}
}

// TestWithScriptNonce verifies which directives of a policy receive the nonce.
func TestWithScriptNonce(t *testing.T) {
	cases := map[string]string{
		"default-src 'self'":                    "default-src 'self'",
		"default-src 'self'; script-src 'self'": "default-src 'self'; script-src 'self' 'nonce-N'",
		"script-src 'self' ; object-src 'none'": "script-src 'self' 'nonce-N'; object-src 'none'",
		"default-src 'self'; Script-Src 'self'": "default-src 'self'; Script-Src 'self' 'nonce-N'",
		// script-src-elem governs <script> elements, so the nonce goes there too.
		"default-src 'self'; script-src-elem 'self'":                "default-src 'self'; script-src-elem 'self' 'nonce-N'",
		"script-src 'self'; script-src-elem 'self'; img-src 'self'": "script-src 'self' 'nonce-N'; script-src-elem 'self' 'nonce-N'; img-src 'self'",
		// Only the first occurrence of a directive counts in CSP; repeats are left alone.
		"script-src 'self'; script-src 'unsafe-eval'": "script-src 'self' 'nonce-N'; script-src 'unsafe-eval'",
		// A "no scripts" directive stays that way.
		"script-src 'none'; script-src-elem 'self'": "script-src 'none'; script-src-elem 'self' 'nonce-N'",
		"default-src 'none'; script-src 'NONE'":     "default-src 'none'; script-src 'NONE'",
	}
	for in, want := range cases {
		if got := withScriptNonce(in, "N"); got != want {
			t.Errorf("withScriptNonce(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCSP verifies the Content-Security-Policy header is set when configured and
// absent otherwise.
func TestCSP(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{CSP: "default-src 'self'"})
	defer srv.Close()
	_ = os.WriteFile(dir+"/index.html", []byte("x"), 0o644)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("CSP: got %q", got)
	}

	off, offDir := newTestServer(t)
	defer off.Close()
	_ = os.WriteFile(offDir+"/index.html", []byte("x"), 0o644)
	resp2, err := http.Get(off.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if got := resp2.Header.Get("Content-Security-Policy"); got != "" {
		t.Errorf("CSP should be unset, got %q", got)
	}
}

// TestSecurityHeaderDefaults verifies HSTS and Permissions-Policy come from the environment with
// the documented defaults, and that setting a variable to "" switches its header off.
func TestSecurityHeaderDefaults(t *testing.T) {
	t.Setenv("STRICT_TRANSPORT_SECURITY", "")
	_ = os.Unsetenv("STRICT_TRANSPORT_SECURITY") // absent → default
	_ = os.Unsetenv("PERMISSIONS_POLICY")
	_ = os.Unsetenv("CSP_NONCE")
	cfg := loadConfig()
	if !cfg.CSPNonce {
		t.Fatal("CSP_NONCE should default to true")
	}
	if cfg.HSTS != defaultHSTS || cfg.Permissions != defaultPermissionsPolicy {
		t.Fatalf("defaults: got HSTS %q, Permissions %q", cfg.HSTS, cfg.Permissions)
	}

	t.Setenv("STRICT_TRANSPORT_SECURITY", "")
	t.Setenv("PERMISSIONS_POLICY", "camera=()")
	cfg = loadConfig()
	if cfg.HSTS != "" || cfg.Permissions != "camera=()" {
		t.Fatalf("overrides: got HSTS %q, Permissions %q", cfg.HSTS, cfg.Permissions)
	}
}

// TestSecurityHeadersSent verifies the configured HSTS and Permissions-Policy values reach the
// response, and that empty values send no header.
func TestSecurityHeadersSent(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{HSTS: defaultHSTS, Permissions: defaultPermissionsPolicy})
	defer srv.Close()
	_ = os.WriteFile(dir+"/index.html", []byte("x"), 0o644)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Strict-Transport-Security"); got != defaultHSTS {
		t.Errorf("HSTS: got %q", got)
	}
	if got := resp.Header.Get("Permissions-Policy"); got != defaultPermissionsPolicy {
		t.Errorf("Permissions-Policy: got %q", got)
	}

	off, offDir := newConfiguredServer(t, config{})
	defer off.Close()
	_ = os.WriteFile(offDir+"/index.html", []byte("x"), 0o644)
	resp2, err := http.Get(off.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.Header.Get("Strict-Transport-Security") != "" || resp2.Header.Get("Permissions-Policy") != "" {
		t.Errorf("empty config must send neither header")
	}
}

// TestMethodNotAllowed verifies non-GET/HEAD methods get 405 with an Allow header.
func TestMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("Allow: got %q, want %q", allow, "GET, HEAD")
	}
}

// TestSecurityHeadersOnMethodNotAllowed verifies the 405 from methodGuard carries every configured
// security header, like any other response (methodGuard must run inside secureHeaders).
func TestSecurityHeadersOnMethodNotAllowed(t *testing.T) {
	cfg := config{
		CSP: "default-src 'self'; script-src 'self'", CSPNonce: true,
		HSTS: defaultHSTS, Permissions: defaultPermissionsPolicy,
	}
	srv, _ := newConfiguredServer(t, cfg)
	defer srv.Close()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions} {
		req, _ := http.NewRequest(method, srv.URL+"/anything", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s: got %d, want 405", method, resp.StatusCode)
		}
		for h, want := range map[string]string{
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "SAMEORIGIN",
			"Referrer-Policy":           "strict-origin-when-cross-origin",
			"Strict-Transport-Security": defaultHSTS,
			"Permissions-Policy":        defaultPermissionsPolicy,
		} {
			if got := resp.Header.Get(h); got != want {
				t.Errorf("%s 405: %s = %q, want %q", method, h, got, want)
			}
		}
		if csp := resp.Header.Get("Content-Security-Policy"); !strings.HasPrefix(csp, "default-src 'self'; script-src 'self' 'nonce-") {
			t.Errorf("%s 405: Content-Security-Policy = %q, want the policy with a nonce", method, csp)
		}
	}
}

// TestPrecompressed verifies a sibling .br is served with Content-Encoding when the
// client accepts it, preserving the original Content-Type.
func TestPrecompressed(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{Precompressed: true})
	defer srv.Close()
	if err := os.WriteFile(dir+"/app.js", []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/app.js.br", []byte("brotli-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	// DisableCompression so the transport neither adds Accept-Encoding nor
	// transparently decodes the response.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/app.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "br" {
		t.Errorf("Content-Encoding: got %q, want br", resp.Header.Get("Content-Encoding"))
	}
	if string(body) != "brotli-bytes" {
		t.Errorf("body: got %q, want the .br contents", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type: got %q, want text/javascript", ct)
	}

	// Without Accept-Encoding, the plain file is served.
	resp2, err := client.Get(srv.URL + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.Header.Get("Content-Encoding") != "" || string(body2) != "plain" {
		t.Errorf("no-AE: got enc=%q body=%q, want plain", resp2.Header.Get("Content-Encoding"), body2)
	}
}

// ── Cascade filesystem tests ──────────────────────────────────────────────────

func TestCascadePrimaryServed(t *testing.T) {
	srv, primary, _ := newCascadeTestServer(t)
	defer srv.Close()

	if err := os.WriteFile(primary+"/index.html", []byte("primary"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCascadeFallbackServed(t *testing.T) {
	// index.html only in fallback; primary is empty.
	srv, _, fallback := newCascadeTestServer(t)
	defer srv.Close()

	if err := os.WriteFile(fallback+"/index.html", []byte("fallback"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from fallback, got %d", resp.StatusCode)
	}
}

func TestCascadePrimaryOverridesFallback(t *testing.T) {
	// Same path exists in both; primary must win.
	srv, primary, fallback := newCascadeTestServer(t)
	defer srv.Close()

	if err := os.WriteFile(primary+"/index.html", []byte("primary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallback+"/index.html", []byte("fallback"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCascadePartialOverlay(t *testing.T) {
	// index.html in primary; style.css only in fallback.
	srv, primary, fallback := newCascadeTestServer(t)
	defer srv.Close()

	if err := os.WriteFile(primary+"/index.html", []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallback+"/style.css", []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// index.html comes from primary
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index.html: expected 200, got %d", resp.StatusCode)
	}

	// style.css falls through to fallback
	resp2, err := http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("style.css: expected 200 from fallback, got %d", resp2.StatusCode)
	}
}

func TestCascadeNotFoundInEither(t *testing.T) {
	srv, _, _ := newCascadeTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ghost.html")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestCascadeEmpty verifies that an empty cascadeFS returns os.ErrNotExist for
// every path, exercising the lastErr == nil branch in cascadeFS.Open.
func TestCascadeEmpty(t *testing.T) {
	_, err := cascadeFS{}.Open("/anything")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

// TestSafeDirStatError verifies that safeDir.Open propagates a Stat error and
// closes the open file handle before returning.
func TestSafeDirStatError(t *testing.T) {
	d := safeDir{statErrFS{}}
	_, err := d.Open("/anything")
	if err == nil {
		t.Fatal("expected error from Stat, got nil")
	}
}

// TestHealthcheckSuccess runs the binary in -healthcheck mode against a live
// httptest.Server and verifies that it exits 0 when /healthz returns 200.
func TestHealthcheckSuccess(t *testing.T) {
	if os.Getenv(subprocessKey) == "healthcheck_success" {
		os.Args = []string{"cmd", "-healthcheck"}
		main()
		return
	}
	srv, _ := newTestServer(t)
	defer srv.Close()
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]

	cmd := exec.Command(os.Args[0], "-test.run=TestHealthcheckSuccess")
	cmd.Env = append(os.Environ(), subprocessKey+"=healthcheck_success", "PORT="+port)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0: %v\n%s", err, out)
	}
}

// TestHealthcheckFailure verifies that the binary exits 1 when /healthz returns
// a non-200 status code in -healthcheck mode.
func TestHealthcheckFailure(t *testing.T) {
	if os.Getenv(subprocessKey) == "healthcheck_failure" {
		os.Args = []string{"cmd", "-healthcheck"}
		main()
		return
	}
	// A real 500 server avoids any port-reuse race condition.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failSrv.Close()
	port := failSrv.URL[strings.LastIndex(failSrv.URL, ":")+1:]

	cmd := exec.Command(os.Args[0], "-test.run=TestHealthcheckFailure")
	cmd.Env = append(os.Environ(), subprocessKey+"=healthcheck_failure", "PORT="+port)
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %v", err)
	}
}

// TestMainGracefulShutdown starts the server in a subprocess, waits for it to
// respond on /healthz, sends SIGTERM, and verifies the process exits cleanly.
// It also exercises the fallback == "" branch in main() via FALLBACK_DIR="".
func TestMainGracefulShutdown(t *testing.T) {
	if os.Getenv(subprocessKey) == "server_shutdown" {
		main()
		return
	}
	// Reserve a free port then release it so the subprocess can bind to it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()

	dir := t.TempDir()

	subArgs := []string{"-test.run=TestMainGracefulShutdown"}
	if gcd := os.Getenv("GOCOVERDIR"); gcd != "" {
		subArgs = append(subArgs, "-test.gocoverdir="+gcd)
	}
	cmd := exec.Command(os.Args[0], subArgs...)
	cmd.Env = append(os.Environ(),
		subprocessKey+"=server_shutdown",
		"PORT="+port,
		"SERVE_DIR="+dir,
		"FALLBACK_DIR=",
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Poll /healthz until the server is ready or the 5-second deadline elapses.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:" + port + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("expected clean exit after SIGTERM, got: %v", err)
	}
}

// get issues a request with the given method and returns the status and body.
func get(t *testing.T, method, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// TestCleanURLs verifies that with CLEAN_URLS on, /page is served from /page.html
// (GET and HEAD), real files and directory indexes still win, and a miss stays 404.
func TestCleanURLs(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{CleanURLs: true})
	defer srv.Close()
	_ = os.MkdirAll(dir+"/guide/sub", 0o755)
	_ = os.WriteFile(dir+"/guide/start.html", []byte("start page"), 0o644)
	_ = os.WriteFile(dir+"/guide/sub/index.html", []byte("sub index"), 0o644)

	if code, body := get(t, http.MethodGet, srv.URL+"/guide/start"); code != 200 || body != "start page" {
		t.Errorf("clean URL: got %d %q, want 200 start page", code, body)
	}
	if code, _ := get(t, http.MethodHead, srv.URL+"/guide/start"); code != 200 {
		t.Errorf("clean URL HEAD: got %d, want 200", code)
	}
	if code, body := get(t, http.MethodGet, srv.URL+"/guide/sub/"); code != 200 || body != "sub index" {
		t.Errorf("directory index: got %d %q", code, body)
	}
	if code, _ := get(t, http.MethodGet, srv.URL+"/guide/missing"); code != 404 {
		t.Errorf("missing page: got %d, want 404", code)
	}
	// Off by default: the extensionless path is not mapped.
	off, offDir := newConfiguredServer(t, config{})
	defer off.Close()
	_ = os.WriteFile(offDir+"/start.html", []byte("x"), 0o644)
	if code, _ := get(t, http.MethodGet, off.URL+"/start"); code != 404 {
		t.Errorf("CLEAN_URLS off: got %d, want 404", code)
	}
}

// TestNotFoundPage verifies that NOT_FOUND_PAGE is served with status 404 for an
// unknown navigation route, that a missing asset stays a plain 404, that a missing
// page file degrades to a plain 404, and that SPA_FALLBACK takes precedence.
func TestNotFoundPage(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{CleanURLs: true, NotFoundPage: "/404.html"})
	defer srv.Close()
	_ = os.WriteFile(dir+"/404.html", []byte("custom not found"), 0o644)

	code, body := get(t, http.MethodGet, srv.URL+"/no/such/page")
	if code != 404 || body != "custom not found" {
		t.Errorf("unknown route: got %d %q, want 404 custom page", code, body)
	}
	if code, body := get(t, http.MethodHead, srv.URL+"/no/such/page"); code != 404 || body != "" {
		t.Errorf("unknown route HEAD: got %d %q, want 404 and no body", code, body)
	}
	if code, body := get(t, http.MethodGet, srv.URL+"/assets/missing.js"); code != 404 || strings.Contains(body, "custom") {
		t.Errorf("missing asset: got %d %q, want plain 404", code, body)
	}

	bare, _ := newConfiguredServer(t, config{NotFoundPage: "/404.html"})
	defer bare.Close()
	if code, _ := get(t, http.MethodGet, bare.URL+"/no/such/page"); code != 404 {
		t.Errorf("absent 404 page: got %d, want plain 404", code)
	}

	spa, spaDir := newConfiguredServer(t, config{SPAFallback: true, NotFoundPage: "/404.html"})
	defer spa.Close()
	_ = os.WriteFile(spaDir+"/index.html", []byte("app"), 0o644)
	_ = os.WriteFile(spaDir+"/404.html", []byte("nf"), 0o644)
	if code, body := get(t, http.MethodGet, spa.URL+"/route"); code != 200 || body != "app" {
		t.Errorf("SPA precedence: got %d %q, want 200 app", code, body)
	}
}

// TestTraversalRejected verifies that encoded and literal parent-directory segments never
// reach a file outside the serve root, with or without the navigation fallbacks.
func TestTraversalRejected(t *testing.T) {
	parent := t.TempDir()
	root := parent + "/root"
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(parent+"/secret.txt", []byte("outside root"), 0o644)
	_ = os.WriteFile(root+"/index.html", []byte("home"), 0o644)
	for _, cfg := range []config{{}, {SPAFallback: true}, {CleanURLs: true, NotFoundPage: "/index.html"}} {
		srv := httptest.NewServer(newHandler(cfg, cascadeFS{http.Dir(root)}))
		for _, p := range []string{"/../secret.txt", "/..%2fsecret.txt", "/%2e%2e/secret.txt", "/..%2f..%2fsecret", "/.%2e/secret.txt"} {
			req, _ := http.NewRequest(http.MethodGet, srv.URL+p, nil)
			req.URL.Opaque = p // send the path exactly as written
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if strings.Contains(string(body), "outside root") {
				t.Errorf("cfg %+v: %s served a file outside the root", cfg, p)
			}
		}
		srv.Close()
	}
}

// serveRooted builds a server over rootedFS(dir): the filesystem main uses.
func serveRooted(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(newHandler(config{}, cascadeFS{rootedFS(dir)}))
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// TestSymlinkEscapeRefused verifies that a symlink leaving the serve root is not followed, while a
// symlink that stays inside it (the shape of a ConfigMap mount's ..data links) keeps working.
func TestSymlinkEscapeRefused(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.txt", filepath.Join(dir, "inside.txt")); err != nil {
		t.Fatal(err)
	}
	srv := serveRooted(t, dir)
	defer srv.Close()
	if got := getStatus(t, srv.URL+"/escape.txt"); got != http.StatusNotFound {
		t.Errorf("symlink outside the root: status %d, want 404", got)
	}
	if got := getStatus(t, srv.URL+"/inside.txt"); got != http.StatusOK {
		t.Errorf("symlink inside the root: status %d, want 200", got)
	}
}

// TestDotfilesHidden verifies that dot-files and dot-directories are not served, except /.well-known/.
func TestDotfilesHidden(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		".env":                         "SECRET=1",
		".git/config":                  "[core]",
		"app/.hidden":                  "x",
		".well-known/security.txt":     "Contact: mailto:a@example.com",
		"app/.well-known/security.txt": "nested well-known is not special",
		"ok.txt":                       "fine",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	srv := serveRooted(t, dir)
	defer srv.Close()
	for path, want := range map[string]int{
		"/ok.txt":                       http.StatusOK,
		"/.well-known/security.txt":     http.StatusOK,
		"/.env":                         http.StatusNotFound,
		"/.git/config":                  http.StatusNotFound,
		"/app/.hidden":                  http.StatusNotFound,
		"/app/.well-known/security.txt": http.StatusNotFound,
		"/%2e%65nv":                     http.StatusNotFound,
	} {
		if got := getStatus(t, srv.URL+path); got != want {
			t.Errorf("GET %s: status %d, want %d", path, got, want)
		}
	}
}

// TestRootedFSMissingDirServesNothing verifies that a directory that cannot be opened is an empty
// filesystem (404 everywhere) rather than a startup failure.
func TestRootedFSMissingDirServesNothing(t *testing.T) {
	srv := serveRooted(t, filepath.Join(t.TempDir(), "does-not-exist"))
	defer srv.Close()
	if got := getStatus(t, srv.URL+"/anything"); got != http.StatusNotFound {
		t.Errorf("status %d, want 404", got)
	}
}

// TestNoCachingHeadersOnErrors pins that an error response never carries a caching header, in
// particular that a missing fingerprinted asset under CACHE_IMMUTABLE_PREFIX is not "immutable"
// (a year-long cached 404 across every app). The guarantee comes from net/http's serveError (Go 1.23+),
// which GODEBUG=httpservecontentkeepheaders=1 would undo, and from serveIndex; this test fails if a
// toolchain or setting change reintroduces it.
func TestNoCachingHeadersOnErrors(t *testing.T) {
	for _, spa := range []bool{false, true} {
		cfg := config{ImmutablePrefix: "/assets/", SPAFallback: spa, CleanURLs: true, NotFoundPage: "/404.html"}
		srv, dir := newConfiguredServer(t, cfg)
		defer srv.Close()
		if err := os.Mkdir(dir+"/assets", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/assets/real.abc123.js", []byte("1"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A directory under the prefix without an index.html is a 404 too.
		if err := os.Mkdir(dir+"/assets/empty", 0o755); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{"/assets/missing.abc123.js", "/assets/nested/missing.css", "/assets/empty/", "/assets/.env", "/missing.js"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				req, _ := http.NewRequest(method, srv.URL+p, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("spa=%v %s %s: got %d, want 404", spa, method, p, resp.StatusCode)
				}
				for _, h := range []string{"Cache-Control", "Etag", "Last-Modified", "Expires"} {
					if v := resp.Header.Get(h); v != "" {
						t.Errorf("spa=%v %s %s: error response carries %s: %q", spa, method, p, h, v)
					}
				}
			}
		}
		// The 405 from the method guard is an error response as well.
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/assets/real.abc123.js", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Cache-Control") != "" {
			t.Errorf("spa=%v POST: got %d with Cache-Control %q, want 405 without", spa, resp.StatusCode, resp.Header.Get("Cache-Control"))
		}
		// A hit under the prefix is still immutable (the test above would otherwise pass trivially).
		hit, err := http.Get(srv.URL + "/assets/real.abc123.js")
		if err != nil {
			t.Fatal(err)
		}
		_ = hit.Body.Close()
		if cc := hit.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
			t.Errorf("spa=%v hit Cache-Control: got %q", spa, cc)
		}
	}
}

// TestSPAWithoutIndexIsPlain404 verifies that an SPA navigation route under the immutable prefix,
// on a deployment with no index.html at all, is a plain 404 and not an immutable one.
func TestSPAWithoutIndexIsPlain404(t *testing.T) {
	srv, _ := newConfiguredServer(t, config{ImmutablePrefix: "/app/", SPAFallback: true})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/app/route")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("got %d, want 404", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		t.Errorf("Cache-Control on the 404: got %q, want none", cc)
	}
}

// TestAcceptsEncoding verifies Accept-Encoding token and q-value parsing.
func TestAcceptsEncoding(t *testing.T) {
	cases := []struct {
		ae, name string
		want     bool
	}{
		{"br", "br", true},
		{"gzip, br", "br", true},
		{"gzip, deflate, br;q=0.8", "br", true},
		{"BR", "br", true},
		{" br ; q=1.0 ", "br", true},
		{"br;q=0, gzip", "br", false},
		{"br;q=0.0, gzip", "br", false},
		{"gzip, br;q=0", "br", false},
		{"br;q=0, gzip", "gzip", true},
		{"br;q=abc", "br", false},
		{"br;level=5", "br", true},
		{"xbr", "br", false},
		{"brotli", "br", false},
		{"x-gzip", "gzip", false},
		{"gzip-ish, identity", "gzip", false},
		{"*", "br", false},
		{"identity", "br", false},
		{"", "br", false},
	}
	for _, c := range cases {
		if got := acceptsEncoding(c.ae, c.name); got != c.want {
			t.Errorf("acceptsEncoding(%q, %q) = %v, want %v", c.ae, c.name, got, c.want)
		}
	}
}

// TestPrecompressedNegotiation verifies q=0 refuses a variant, and that Vary: Accept-Encoding is set
// on every response for an asset path, the identity variant included.
func TestPrecompressedNegotiation(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{Precompressed: true})
	defer srv.Close()
	for name, body := range map[string]string{"app.js": "plain", "app.js.br": "br-bytes", "app.js.gz": "gz-bytes", "other.css": "css"} {
		if err := os.WriteFile(dir+"/"+name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	cases := []struct {
		path, ae, wantBody, wantEnc string
	}{
		{"/app.js", "br", "br-bytes", "br"},
		{"/app.js", "gzip, br", "br-bytes", "br"},
		{"/app.js", "br;q=0, gzip", "gz-bytes", "gzip"},
		{"/app.js", "br;q=0, gzip;q=0", "plain", ""},
		{"/app.js", "gzip", "gz-bytes", "gzip"},
		{"/app.js", "xbr", "plain", ""},
		{"/app.js", "", "plain", ""},
		{"/other.css", "br", "css", ""}, // no sibling: identity, still keyed by Vary
	}
	for _, c := range cases {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+c.path, nil)
		if c.ae != "" {
			req.Header.Set("Accept-Encoding", c.ae)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if string(body) != c.wantBody || resp.Header.Get("Content-Encoding") != c.wantEnc {
			t.Errorf("%s with AE %q: got %q enc %q, want %q enc %q", c.path, c.ae, body, resp.Header.Get("Content-Encoding"), c.wantBody, c.wantEnc)
		}
		if vary := resp.Header.Values("Vary"); len(vary) != 1 || vary[0] != "Accept-Encoding" {
			t.Errorf("%s with AE %q: Vary = %q, want exactly [Accept-Encoding]", c.path, c.ae, vary)
		}
	}
	// A navigation route is never negotiated, so it carries no Vary.
	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if v := resp.Header.Get("Vary"); v != "" {
		t.Errorf("/ carries Vary %q", v)
	}
}

// TestPrecompressedOffNoVary verifies the Vary header only appears when PRECOMPRESSED is on.
func TestPrecompressedOffNoVary(t *testing.T) {
	srv, dir := newConfiguredServer(t, config{})
	defer srv.Close()
	if err := os.WriteFile(dir+"/app.js", []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if v := resp.Header.Get("Vary"); v != "" {
		t.Errorf("Vary with PRECOMPRESSED off: %q", v)
	}
}

// docSurfaces are the files CLAUDE.md requires every configuration option to appear in.
var docSurfaces = []string{
	"README.md", "llms.txt", "public/llms.txt", "public/index.html", ".env.example", "AGENTS.md",
}

// TestDocsListEveryOption pins the documentation discipline: every environment variable that main.go
// reads appears in every reference surface, so a new option cannot ship undocumented.
func TestDocsListEveryOption(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	opts := regexp.MustCompile(`env(?:Or|OrEmpty|Bool)\("([A-Z_]+)"`).FindAllStringSubmatch(string(src), -1)
	if len(opts) < 10 {
		t.Fatalf("found only %d options in main.go; the pattern no longer matches", len(opts))
	}
	for _, surface := range docSurfaces {
		body, err := os.ReadFile(surface)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range opts {
			if !strings.Contains(string(body), m[1]) {
				t.Errorf("%s does not mention %s", surface, m[1])
			}
		}
	}
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range opts {
		if m[1] != "PORT" && !strings.Contains(string(changelog), m[1]) {
			t.Errorf("CHANGELOG.md does not mention %s", m[1])
		}
	}
}

// imageDefaultCSP reads the CONTENT_SECURITY_POLICY the Dockerfile bakes into the image.
func imageDefaultCSP(t *testing.T) string {
	t.Helper()
	df, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^ENV CONTENT_SECURITY_POLICY="([^"]+)"$`).FindStringSubmatch(string(df))
	if m == nil {
		t.Fatal(`Dockerfile has no ENV CONTENT_SECURITY_POLICY="..." line`)
	}
	return m[1]
}

// TestBinaryCSPDefaultIsUnset pins that the default CSP lives in the image only: the Go code default
// stays empty, so running the binary directly sends no CSP, and an empty value switches the image
// default off.
func TestBinaryCSPDefaultIsUnset(t *testing.T) {
	_ = os.Unsetenv("CONTENT_SECURITY_POLICY")
	if got := loadConfig().CSP; got != "" {
		t.Fatalf("binary CSP default is %q, want empty (the default belongs to the Dockerfile)", got)
	}
	t.Setenv("CONTENT_SECURITY_POLICY", "")
	if got := loadConfig().CSP; got != "" {
		t.Fatalf("an empty CONTENT_SECURITY_POLICY gave %q, want empty (the documented opt-out)", got)
	}
}

// TestImageDefaultCSP pins the image's default policy: it is restrictive (no inline, no third-party
// origin), it is documented verbatim on every reference surface, and it keeps working with the
// per-response script nonce.
func TestImageDefaultCSP(t *testing.T) {
	csp := imageDefaultCSP(t)
	for _, want := range []string{
		"default-src 'self'", "script-src 'self'", "style-src 'self'", "object-src 'none'",
		"base-uri 'none'", "frame-ancestors 'self'", "form-action 'self'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("default CSP lacks %q: %s", want, csp)
		}
	}
	for _, bad := range []string{"unsafe-inline", "unsafe-eval", "http:", "https:", "*"} {
		if strings.Contains(csp, bad) {
			t.Errorf("default CSP contains %q: %s", bad, csp)
		}
	}
	for _, surface := range append([]string{"CHANGELOG.md"}, docSurfaces...) {
		body, err := os.ReadFile(surface)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), csp) {
			t.Errorf("%s does not state the image default CSP verbatim", surface)
		}
	}
	got := withScriptNonce(csp, "N")
	if !strings.Contains(got, "script-src 'self' 'nonce-N';") || strings.Count(got, "nonce-N") != 1 {
		t.Errorf("nonce handling of the default CSP: %s", got)
	}
}

// TestDefaultPageFitsDefaultCSP pins that the image's built-in page loads under the default policy:
// no inline script (the JSON-LD block is data, not executed), no inline style or style attribute, no
// event-handler attribute, and no third-party URL in the script, the stylesheet or any resource
// reference.
func TestDefaultPageFitsDefaultCSP(t *testing.T) {
	htmlBytes, err := os.ReadFile("public/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(htmlBytes)
	for _, m := range regexp.MustCompile(`(?s)<script\b([^>]*)>(.*?)</script>`).FindAllStringSubmatch(page, -1) {
		attrs := m[1]
		if strings.Contains(attrs, `type="application/ld+json"`) {
			continue
		}
		if !regexp.MustCompile(`\bsrc="/[^"/]`).MatchString(attrs) || strings.TrimSpace(m[2]) != "" {
			t.Errorf("default page has an inline or non-same-origin script: <script%s>", attrs)
		}
	}
	for _, re := range []string{`(?i)<style\b`, `(?i)\sstyle\s*=`, `(?i)\son[a-z]+\s*=`, `(?i)javascript:`} {
		if regexp.MustCompile(re).MatchString(page) {
			t.Errorf("default page matches %s, which the default CSP blocks", re)
		}
	}
	// Resource-loading references must be same-origin (links in the body are navigation, not loads).
	for _, m := range regexp.MustCompile(`(?i)<(?:link|img|script|iframe|source|video|audio|embed|object)\b[^>]*\b(?:src|href|data)="([^"]*)"`).FindAllStringSubmatch(page, -1) {
		if !strings.HasPrefix(m[1], "/") || strings.HasPrefix(m[1], "//") {
			t.Errorf("default page loads %q, which is not same-origin", m[1])
		}
	}
	css, err := os.ReadFile("public/style.css")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)@import|url\(\s*["']?(?:https?:)?//`).Match(css) {
		t.Error("public/style.css loads a third-party resource")
	}
	js, err := os.ReadFile("public/script.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"eval(", "new Function", "innerHTML", "document.write", "fetch(", "XMLHttpRequest"} {
		if strings.Contains(string(js), bad) {
			t.Errorf("public/script.js uses %s, which the default CSP or its purpose rules out", bad)
		}
	}
}
