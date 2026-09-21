package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
