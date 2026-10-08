// Package main implements a minimal HTTP static file server for use in scratch
// container images, with first-class support for hosting single-page apps (SPAs).
//
// The server resolves files from a primary directory (SERVE_DIR) and optionally
// falls through to a secondary fallback directory (FALLBACK_DIR), enabling partial
// volume overlays in Kubernetes without requiring operators to supply every file.
// Security response headers are applied on every request, directory listing is
// disabled, only GET/HEAD are accepted, and the server shuts down gracefully on
// SIGTERM or SIGINT.
//
// # Endpoints
//
//	GET /          serve SERVE_DIR/index.html; fall through to FALLBACK_DIR on miss
//	GET /<path>    serve static file from SERVE_DIR, fall through to FALLBACK_DIR on miss
//	GET /healthz   liveness and readiness probe; responds 200 "ok"
//	GET /version   build-time version string as plain text
//
// # Runtime configuration
//
// All configuration is via environment variables. Every variable has a built-in
// default; set only the values that differ from the defaults.
//
//	PORT                    TCP listen port (default: 8080)
//	SERVE_DIR               primary serve directory (default: /app/public)
//	FALLBACK_DIR            fallback directory; empty disables the cascade (default: /app/default)
//	SPA_FALLBACK            serve index.html for unknown navigation routes (default: false)
//	CLEAN_URLS              serve /<path> from /<path>.html when /<path> itself does not exist, for
//	                        static-site generators that link extensionless URLs (default: false)
//	NOT_FOUND_PAGE          page served with status 404 for unknown navigation routes, e.g.
//	                        /404.html (default: "" = plain 404; ignored when SPA_FALLBACK is on)
//	CACHE_IMMUTABLE_PREFIX  path prefix whose files get long immutable caching (default: "" = off)
//	CONTENT_SECURITY_POLICY value of the Content-Security-Policy header (default: "" = unset)
//	CSP_NONCE               add a fresh 'nonce-…' to the CSP's script-src on every response, so an
//	                        edge (e.g. Cloudflare JavaScript Detections) can mark the inline scripts
//	                        it injects; the app's own scripts stay governed by 'self' (default: true)
//	STRICT_TRANSPORT_SECURITY value of the Strict-Transport-Security header
//	                        (default: "max-age=31536000; includeSubDomains"; set to "" to omit)
//	PERMISSIONS_POLICY      value of the Permissions-Policy header
//	                        (default: "camera=(), geolocation=(), microphone=(), payment=()";
//	                        set to "" to omit)
//	PRECOMPRESSED           serve sibling .br/.gz assets when accepted (default: false)
//
// # One-shot healthcheck mode
//
// When invoked with the -healthcheck flag, the binary dials localhost:PORT/healthz
// and exits 0 on HTTP 200 or 1 on any error. This mode is used by the Dockerfile
// HEALTHCHECK CMD instruction since scratch images have no wget or curl.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

// version is the build-time version string reported by GET /version and logged at
// startup. It defaults to "dev" and is set at link time with:
//
//	-ldflags "-X main.version=v0.1.0"
var version = "dev"

// Default security headers. HSTS: one year, subdomains included (every *.meddleware.co.uk host is
// HTTPS-only behind Cloudflare); no `preload` — joining the preload list is an irreversible,
// operator-level decision. Permissions-Policy: none of these apps use the camera, location,
// microphone or payment APIs, so deny them to every frame.
const (
	defaultHSTS              = "max-age=31536000; includeSubDomains"
	defaultPermissionsPolicy = "camera=(), geolocation=(), microphone=(), payment=()"
)

// config holds the runtime configuration loaded from the environment. It is built
// once in main (or per-test) and threaded through newHandler into the middlewares.
type config struct {
	Port            string // TCP listen port
	ServeDir        string // primary serve directory
	FallbackDir     string // fallback directory ("" disables the cascade)
	SPAFallback     bool   // serve index.html for unknown navigation routes
	CleanURLs       bool   // serve /<path> from /<path>.html when /<path> does not exist
	NotFoundPage    string // page served with 404 for unknown navigation routes ("" = plain 404)
	ImmutablePrefix string // path prefix whose files get immutable caching ("" = off)
	CSP             string // Content-Security-Policy header value ("" = unset)
	CSPNonce        bool   // add a per-response nonce to the CSP's script-src
	HSTS            string // Strict-Transport-Security header value ("" = unset)
	Permissions     string // Permissions-Policy header value ("" = unset)
	Precompressed   bool   // serve sibling .br/.gz assets when the client accepts them
}

// loadConfig reads the full configuration from the environment, applying the
// built-in defaults documented in the package comment.
func loadConfig() config {
	return config{
		Port:            envOr("PORT", "8080"),
		ServeDir:        envOr("SERVE_DIR", "/app/public"),
		FallbackDir:     envOr("FALLBACK_DIR", ""),
		SPAFallback:     envBool("SPA_FALLBACK", false),
		CleanURLs:       envBool("CLEAN_URLS", false),
		NotFoundPage:    envOr("NOT_FOUND_PAGE", ""),
		ImmutablePrefix: envOr("CACHE_IMMUTABLE_PREFIX", ""),
		CSP:             envOr("CONTENT_SECURITY_POLICY", ""),
		CSPNonce:        envBool("CSP_NONCE", true),
		HSTS:            envOrEmpty("STRICT_TRANSPORT_SECURITY", defaultHSTS),
		Permissions:     envOrEmpty("PERMISSIONS_POLICY", defaultPermissionsPolicy),
		Precompressed:   envBool("PRECOMPRESSED", false),
	}
}

// init registers content types that a scratch image cannot infer, because it has
// no /etc/mime.types. Without this, assets such as WebAssembly modules would be
// served as application/octet-stream and fail streaming instantiation. Registering
// at init time means both the server and the tests observe the same types.
func init() {
	for ext, typ := range map[string]string{
		".wasm":  "application/wasm",
		".js":    "text/javascript; charset=utf-8",
		".mjs":   "text/javascript; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".json":  "application/json",
		".map":   "application/json",
		".svg":   "image/svg+xml",
		".webp":  "image/webp",
		".woff2": "font/woff2",
		".ico":   "image/x-icon",
	} {
		_ = mime.AddExtensionType(ext, typ)
	}
}

// main is the program entry point. It operates in one of two modes depending on
// command-line arguments.
//
// # Healthcheck mode
//
// Invoked as: server -healthcheck
//
// main dials http://localhost:PORT/healthz. It exits 0 if the response status is
// HTTP 200 and exits 1 on any network error or non-200 response. This mode is
// intended for use by the Dockerfile HEALTHCHECK CMD instruction.
//
// # Server mode (default)
//
// main loads the configuration, constructs the cascade filesystem and handler
// stack via newHandler, installs a SIGTERM/SIGINT handler for graceful shutdown
// with a 10-second drain window, and calls ListenAndServe.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if len(os.Args) == 2 && os.Args[1] == "-healthcheck" {
		os.Exit(healthCheck())
	}

	cfg := loadConfig()

	// Build the cascade filesystem: SERVE_DIR is always tried first. FALLBACK_DIR
	// is appended only when non-empty so that setting FALLBACK_DIR="" completely
	// disables the fallback behaviour.
	fsList := []http.FileSystem{rootedFS(cfg.ServeDir)}
	if cfg.FallbackDir != "" {
		fsList = append(fsList, rootedFS(cfg.FallbackDir))
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      newHandler(cfg, cascadeFS(fsList)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		slog.Info("shutting down", "version", version)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "err", err)
		}
	}()

	slog.Info("listening", "version", version, "serve_dir", cfg.ServeDir,
		"fallback_dir", cfg.FallbackDir, "port", cfg.Port,
		"spa_fallback", cfg.SPAFallback, "precompressed", cfg.Precompressed)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

// healthCheck dials the local /healthz endpoint and returns a process exit code
// (0 healthy, 1 unhealthy) for the container HEALTHCHECK instruction.
func healthCheck() int {
	port := envOr("PORT", "8080")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:"+port+"/healthz", nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// newHandler builds the full handler stack over the given cascade filesystem. It
// is the single source of truth for request handling, used by both main and the
// tests. The chain is: logRequest → methodGuard → secureHeaders → mux, where the
// "/" route is served by fileHandler (cache-control + optional precompressed +
// optional SPA fallback around http.FileServer(safeDir{fs})).
func newHandler(cfg config, fs http.FileSystem) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/version", handleVersion)
	mux.Handle("/", fileHandler(cfg, fs))
	return logRequest(methodGuard(secureHeaders(cfg, mux)))
}

// handleHealthz responds 200 "ok" to liveness and readiness probes.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleVersion writes the build-time version string as plain text.
func handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(version + "\n"))
}

// fileHandler serves static files from the cascade filesystem fs. It applies
// Cache-Control (setCacheControl), optionally serves a precompressed sibling
// (tryPrecompressed), optionally falls back to index.html for unknown navigation
// routes when SPA_FALLBACK is enabled, and otherwise delegates to the standard
// http.FileServer wrapped in safeDir (which disables directory listing).
func fileHandler(cfg config, fs http.FileSystem) http.Handler {
	sfs := safeDir{noDotfiles{fs}}
	fileServer := http.FileServer(sfs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCacheControl(cfg, w, r.URL.Path)

		if cfg.Precompressed && tryPrecompressed(w, r, fs) {
			return
		}

		if isNavigation(r) && !exists(sfs, r.URL.Path) {
			// A generated page for the clean URL wins over every fallback.
			if cfg.CleanURLs && !strings.HasSuffix(r.URL.Path, "/") &&
				serveFileContent(w, r, fs, path.Clean(r.URL.Path)+".html") {
				return
			}
			if cfg.SPAFallback {
				serveIndex(w, r, fs)
				return
			}
			if cfg.NotFoundPage != "" && serveNotFoundPage(w, r, fs, cfg.NotFoundPage) {
				return
			}
		}

		fileServer.ServeHTTP(w, r)
	})
}

// isNavigation reports whether r looks like a page navigation rather than an asset
// request: a GET or HEAD whose final path segment has no file extension and which is
// not the root "/". Asset requests (paths ending in a real extension) are
// deliberately excluded so a missing asset 404s instead of masking a broken build.
func isNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.URL.Path == "/" {
		return false
	}
	return !strings.Contains(path.Base(r.URL.Path), ".")
}

// exists reports whether p resolves to a servable file or directory-with-index in
// fs (using the same safeDir semantics the file server uses). It is used to decide
// whether the SPA fallback should fire.
func exists(fs http.FileSystem, p string) bool {
	f, err := fs.Open(path.Clean(p))
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// serveIndex serves SERVE_DIR/index.html (via the cascade fs) with a no-cache
// header, using http.ServeContent so conditional and range requests are honoured.
// It is the SPA history-fallback target. A missing or unreadable index yields 404.
func serveIndex(w http.ResponseWriter, r *http.Request, fs http.FileSystem) {
	if !serveFileContent(w, r, fs, "/index.html") {
		http.NotFound(w, r)
	}
}

// openRegular opens p in fs and returns it only when it is a seekable regular file.
func openRegular(fs http.FileSystem, p string) (http.File, os.FileInfo, bool) {
	f, err := fs.Open(p)
	if err != nil {
		return nil, nil, false
	}
	stat, err := f.Stat()
	if _, seekable := f.(io.ReadSeeker); err != nil || stat.IsDir() || !seekable {
		_ = f.Close()
		return nil, nil, false
	}
	return f, stat, true
}

// serveFileContent serves the regular file p from fs with status 200 and a no-cache
// header, honouring conditional and range requests. It reports false, writing
// nothing, when p is not a regular file.
func serveFileContent(w http.ResponseWriter, r *http.Request, fs http.FileSystem, p string) bool {
	f, stat, ok := openRegular(fs, p)
	if !ok {
		return false
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, path.Base(p), stat.ModTime(), f.(io.ReadSeeker))
	return true
}

// serveNotFoundPage serves the HTML page p from fs with status 404, so a static site
// can show its own not-found page while crawlers and link checkers see the miss. It
// reports false, writing nothing, when p is not a regular file.
func serveNotFoundPage(w http.ResponseWriter, r *http.Request, fs http.FileSystem, p string) bool {
	f, _, ok := openRegular(fs, p)
	if !ok {
		return false
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusNotFound)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, f)
	}
	return true
}

// setCacheControl sets a Cache-Control header appropriate to the request path:
// files under cfg.ImmutablePrefix (typically the fingerprinted /assets/ dir) get a
// one-year immutable cache; the root, directory paths, .html files, and extension-
// less navigation routes get no-cache so new deploys are picked up immediately.
// Other paths are left untouched. Callers may override (serveIndex sets no-cache).
func setCacheControl(cfg config, w http.ResponseWriter, p string) {
	if cfg.ImmutablePrefix != "" && strings.HasPrefix(p, cfg.ImmutablePrefix) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	if p == "/" || strings.HasSuffix(p, "/") || strings.HasSuffix(p, ".html") ||
		!strings.Contains(path.Base(p), ".") {
		w.Header().Set("Cache-Control", "no-cache")
	}
}

// tryPrecompressed serves a precompressed sibling of the requested asset when the
// client accepts it and the sibling exists. For a request to /app.js with
// Accept-Encoding: br, it serves /app.js.br with Content-Encoding: br and Vary:
// Accept-Encoding, preserving the original asset's Content-Type. It returns true
// when it has served a response. Only asset paths (with a file extension) are
// considered, so navigation routes are never intercepted here.
func tryPrecompressed(w http.ResponseWriter, r *http.Request, fs http.FileSystem) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	p := path.Clean(r.URL.Path)
	ext := path.Ext(p)
	if ext == "" {
		return false
	}
	ae := r.Header.Get("Accept-Encoding")
	for _, enc := range []struct{ name, suffix string }{{"br", ".br"}, {"gzip", ".gz"}} {
		if !strings.Contains(ae, enc.name) {
			continue
		}
		f, err := fs.Open(p + enc.suffix)
		if err != nil {
			continue
		}
		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			_ = f.Close()
			continue
		}
		rs, ok := f.(io.ReadSeeker)
		if !ok {
			_ = f.Close()
			continue
		}
		if ctype := mime.TypeByExtension(ext); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		w.Header().Set("Content-Encoding", enc.name)
		w.Header().Add("Vary", "Accept-Encoding")
		http.ServeContent(w, r, p, stat.ModTime(), rs)
		_ = f.Close()
		return true
	}
	return false
}

// methodGuard rejects any method other than GET or HEAD with 405 Method Not
// Allowed and an Allow header. static-server is strictly read-only.
func methodGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// cascadeFS is a slice of http.FileSystem values consulted in order for each
// Open call. The first filesystem that opens the named path successfully wins;
// if every filesystem returns an error, the error from the last one is returned.
// An empty cascadeFS returns os.ErrNotExist for every path.
//
// cascadeFS enables partial volume overlays: operators mount only the files they
// want to customise into SERVE_DIR (the first entry) and rely on FALLBACK_DIR
// (the second entry) for files they do not supply. Baked-in image defaults live
// in FALLBACK_DIR and are never overwritten by operator mounts.
type cascadeFS []http.FileSystem

// Open implements http.FileSystem. It iterates over the filesystems in fs and
// returns the result of the first successful Open call. If all filesystems return
// an error, Open returns the error from the last filesystem. If fs is empty, Open
// returns os.ErrNotExist.
func (fs cascadeFS) Open(name string) (http.File, error) {
	var lastErr error
	for _, sys := range fs {
		f, err := sys.Open(name)
		if err == nil {
			return f, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		return nil, os.ErrNotExist
	}
	return nil, lastErr
}

// rootedFS serves dir through os.OpenRoot: every path is resolved inside dir, and a symlink that
// leaves it is refused (http.Dir follows them, so a stray symlink in a mounted volume would serve
// whatever the container can read, such as a service-account token). Symlinks that stay inside dir
// (the ..data links of a ConfigMap mount) keep working. A directory that cannot be opened yields an
// empty filesystem (everything is 404) rather than stopping the server, as http.Dir did.
func rootedFS(dir string) http.FileSystem {
	root, err := os.OpenRoot(dir)
	if err != nil {
		slog.Warn("serve directory cannot be opened; serving nothing from it", "dir", dir, "err", err)
		return cascadeFS{}
	}
	return escapeIsNotFound{http.FS(root.FS())}
}

// escapeIsNotFound reports a path the root refuses (a symlink leaving it) as not found: http.FileServer
// would otherwise answer 500, and a 404 reveals nothing about what lies outside the root.
type escapeIsNotFound struct{ http.FileSystem }

// Open implements http.FileSystem; any error other than not-found or permission becomes os.ErrNotExist.
func (e escapeIsNotFound) Open(name string) (http.File, error) {
	f, err := e.FileSystem.Open(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
		return nil, os.ErrNotExist
	}
	return f, err
}

// noDotfiles hides dot-files and dot-directories (.env, .git/, editor and backup droppings): any
// path segment that starts with "." is not found, except a leading /.well-known/ (RFC 8615).
type noDotfiles struct{ http.FileSystem }

// Open implements http.FileSystem; a hidden path is os.ErrNotExist.
func (d noDotfiles) Open(name string) (http.File, error) {
	segments := strings.Split(strings.Trim(name, "/"), "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ".") && seg != "." && (i != 0 || seg != ".well-known") {
			return nil, os.ErrNotExist
		}
	}
	return d.FileSystem.Open(name)
}

// safeDir wraps an http.FileSystem and prevents directory listing. For any path
// that resolves to a directory, safeDir checks whether an index.html file exists
// in that directory (via the underlying filesystem). If no index.html is found,
// Open returns os.ErrNotExist, causing http.FileServer to respond with 404 rather
// than a directory listing.
//
// safeDir is always placed as the outermost layer, wrapping the full cascadeFS,
// so the index.html check benefits from cascade fallthrough.
type safeDir struct{ http.FileSystem }

// Open implements http.FileSystem with directory listing prevention. If name
// resolves to a directory and no adjacent index.html exists in d.FileSystem,
// Open returns os.ErrNotExist. For all other paths Open delegates directly to
// the embedded FileSystem.
func (d safeDir) Open(name string) (http.File, error) {
	f, err := d.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if stat.IsDir() {
		idx := strings.TrimSuffix(name, "/") + "/index.html"
		idxFile, err := d.FileSystem.Open(idx)
		if err != nil {
			_ = f.Close()
			return nil, os.ErrNotExist
		}
		_ = idxFile.Close()
	}
	return f, nil
}

// secureHeaders is an HTTP middleware that sets conservative security response
// headers on every response, regardless of status code or the downstream handler.
// It always sets X-Content-Type-Options, X-Frame-Options, and Referrer-Policy, and
// additionally sets Content-Security-Policy, Strict-Transport-Security and
// Permissions-Policy when their configured values are non-empty.
func secureHeaders(cfg config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if cfg.CSP != "" {
			csp := cfg.CSP
			if cfg.CSPNonce {
				csp = withScriptNonce(csp, newNonce())
			}
			w.Header().Set("Content-Security-Policy", csp)
		}
		if cfg.HSTS != "" {
			w.Header().Set("Strict-Transport-Security", cfg.HSTS)
		}
		if cfg.Permissions != "" {
			w.Header().Set("Permissions-Policy", cfg.Permissions)
		}
		next.ServeHTTP(w, r)
	})
}

// newNonce returns 128 random bits, base64-encoded, for a CSP nonce. A failing system RNG is
// unrecoverable for a server that issues nonces, so it panics (net/http turns that into a 500).
func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("csp nonce: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(b)
}

// withScriptNonce appends 'nonce-<nonce>' to the policy's script-src directive. A policy without
// script-src is returned unchanged: adding one would replace the default-src fallback for scripts.
// The nonce only lets markup carrying it run; this server never inlines scripts itself, so it serves
// edges that inject scripts and parse the nonce from this header (Cloudflare does).
func withScriptNonce(csp, nonce string) string {
	parts := strings.Split(csp, ";")
	for i, p := range parts {
		fields := strings.Fields(p)
		if len(fields) > 0 && strings.EqualFold(fields[0], "script-src") {
			parts[i] = strings.TrimRight(p, " ") + " 'nonce-" + nonce + "'"
			return strings.Join(parts, ";")
		}
	}
	return csp
}

// responseWriter wraps http.ResponseWriter to capture the HTTP status code written
// by the downstream handler. If WriteHeader is never called by the handler chain,
// code defaults to http.StatusOK (200), matching the behaviour of the standard library.
type responseWriter struct {
	http.ResponseWriter
	code int
}

// WriteHeader records code in rw.code and delegates to the embedded
// http.ResponseWriter. It must be called before any response body is written.
func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

// logRequest is an HTTP middleware that logs the HTTP method, URL path, response
// status code, and total elapsed time for every request via log/slog (JSON). The
// status code is captured by wrapping the response writer in a responseWriter
// before invoking the downstream handler.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path,
			"status", rw.code, "elapsed_ms", time.Since(start).Milliseconds())
	})
}

// envOr returns the value of the environment variable named by key. If the
// variable is unset or empty, envOr returns the provided fallback value.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrEmpty returns the value of the environment variable named by key when it is set — even to
// the empty string, which an operator uses to switch a default header off — and fallback only when
// the variable is absent.
func envOrEmpty(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// envBool returns true when the environment variable named by key is set to
// "1", "true", "yes", or "on" (case-insensitive); otherwise it returns fallback.
func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
