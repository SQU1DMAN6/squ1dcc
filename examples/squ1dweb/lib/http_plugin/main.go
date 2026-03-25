package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
)

type RouteEntry struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
	Action  string `json:"action,omitempty"`
}

type RouteConfig struct {
	Routes []RouteEntry `json:"routes"`
}

type RouteResponse struct {
	Status      int               `json:"status"`
	ContentType string            `json:"content_type"`
	Body        interface{}       `json:"body"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type EngineSettings struct {
	StaticRoot      string `json:"static_root"`
	LogPath         string `json:"log_path"`
	DBPath          string `json:"db_path"`
	SessionPath     string `json:"session_path"`
	SessionCookie   string `json:"session_cookie"`
	SessionTTL      int    `json:"session_ttl_seconds"`
	SecureCookies   bool   `json:"secure_cookies"`
	UploadDirectory string `json:"upload_directory"`
}

type ServerState struct {
	PID        int    `json:"pid"`
	Port       string `json:"port"`
	LogPath    string `json:"log_path"`
	StaticRoot string `json:"static_root"`
	StartedAt  string `json:"started_at"`
	TLS        bool   `json:"tls"`
	CertPath   string `json:"cert_path,omitempty"`
	KeyPath    string `json:"key_path,omitempty"`
}

type SessionRecord struct {
	ID        string      `json:"id"`
	User      interface{} `json:"user"`
	Flash     interface{} `json:"flash,omitempty"`
	CreatedAt string      `json:"created_at"`
	ExpiresAt string      `json:"expires_at"`
}

type SessionStore struct {
	Sessions map[string]SessionRecord `json:"sessions"`
}

type RequestContext struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Route       string            `json:"route"`
	Action      string            `json:"action"`
	Query       string            `json:"query"`
	ContentType string            `json:"content_type"`
	Cookie      string            `json:"cookie"`
	Body        string            `json:"body"`
	BodyFile    string            `json:"body_file"`
	Headers     map[string]string `json:"headers"`
}

var (
	routeConfigPath = ""
	settingsPath    = ""
	serverStatePath = ""
	stateDirPath    = ""
	requestCtxDir   = ""

	routeMutex    sync.RWMutex
	settingsMutex sync.Mutex
	sessionMutex  sync.Mutex
	logMutex      sync.Mutex

	squ1dccPath = os.Getenv("SQU1DCC_BIN")
	projectRoot = ""
	staticRoot  = ""
	logFilePath = ""

	templateRawExprPattern = regexp.MustCompile(`<\?!=\s*([a-zA-Z0-9_.-]+)\s*\?>`)
	templateExprPattern    = regexp.MustCompile(`<\?=\s*([a-zA-Z0-9_.-]+)\s*\?>`)
)

func init() {
	if squ1dccPath == "" {
		if path, err := exec.LookPath("squ1dcc"); err == nil {
			squ1dccPath = path
		} else {
			squ1dccPath = "squ1dcc"
		}
	}

	projectRoot = detectProjectRoot()
	stateDirPath = filepath.Join(projectRoot, ".squ1dweb")
	routeConfigPath = filepath.Join(stateDirPath, "routes.json")
	settingsPath = filepath.Join(stateDirPath, "settings.json")
	serverStatePath = filepath.Join(stateDirPath, "server_state.json")
	requestCtxDir = filepath.Join(stateDirPath, "request_ctx")
	_ = os.MkdirAll(stateDirPath, 0o755)
	_ = os.MkdirAll(requestCtxDir, 0o755)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__server__" {
		if len(os.Args) > 2 {
			startWebServer(os.Args[2])
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "__server_tls__" {
		if len(os.Args) > 4 {
			startWebServerTLS(os.Args[2], os.Args[3], os.Args[4])
		}
		return
	}

	module := NewSQXModule("http")
	module.RegisterMany(
		SQXMethod{Name: "ping", Return: SQXReturnString, Handle: pingHandler},
		SQXMethod{Name: "get", Return: SQXReturnJSON, Handle: getHandler},
		SQXMethod{Name: "post", Return: SQXReturnJSON, Handle: postHandler},
		SQXMethod{Name: "status", Return: SQXReturnInt, Handle: statusHandler},

		SQXMethod{Name: "server_start", Return: SQXReturnString, Handle: serverStartHandler},
		SQXMethod{Name: "server_start_tls", Return: SQXReturnString, Handle: serverStartTLSHandler},
		SQXMethod{Name: "server_stop", Return: SQXReturnString, Handle: serverStopHandler},
		SQXMethod{Name: "server_status", Return: SQXReturnJSON, Handle: serverStatusHandler},

		SQXMethod{Name: "set_route", Return: SQXReturnString, Handle: setRouteHandler},
		SQXMethod{Name: "list_routes", Return: SQXReturnJSON, Handle: listRoutesHandler},
		SQXMethod{Name: "remove_route", Return: SQXReturnString, Handle: removeRouteHandler},
		SQXMethod{Name: "clear_routes", Return: SQXReturnString, Handle: clearRoutesHandler},

		SQXMethod{Name: "set_static_root", Return: SQXReturnString, Handle: setStaticRootHandler},
		SQXMethod{Name: "static_root", Return: SQXReturnString, Handle: staticRootHandler},
		SQXMethod{Name: "set_log_file", Return: SQXReturnString, Handle: setLogFileHandler},
		SQXMethod{Name: "log_path", Return: SQXReturnString, Handle: logPathHandler},
		SQXMethod{Name: "log_tail", Return: SQXReturnString, Handle: logTailHandler},
		SQXMethod{Name: "set_upload_dir", Return: SQXReturnString, Handle: setUploadDirHandler},
		SQXMethod{Name: "upload_dir", Return: SQXReturnString, Handle: uploadDirHandler},
		SQXMethod{Name: "upload_user_dir", Return: SQXReturnJSON, Handle: uploadUserDirHandler},
		SQXMethod{Name: "upload_path", Return: SQXReturnString, Handle: uploadPathHandler},
		SQXMethod{Name: "upload_delete", Return: SQXReturnJSON, Handle: uploadDeleteHandler},
		SQXMethod{Name: "redirect", Return: SQXReturnString, Handle: redirectHandler},

		SQXMethod{Name: "response", Return: SQXReturnString, Handle: responseHandler},
		SQXMethod{Name: "response_json", Return: SQXReturnString, Handle: responseJSONHandler},
		SQXMethod{Name: "response_html", Return: SQXReturnString, Handle: responseHTMLHandler},
		SQXMethod{Name: "response_text", Return: SQXReturnString, Handle: responseTextHandler},
		SQXMethod{Name: "render_file", Return: SQXReturnString, Handle: renderFileHandler},
		SQXMethod{Name: "render_template", Return: SQXReturnString, Handle: renderTemplateHandler},
		SQXMethod{Name: "escape_html", Return: SQXReturnString, Handle: escapeHTMLHandler},
		SQXMethod{Name: "download_file", Return: SQXReturnString, Handle: downloadFileHandler},
		SQXMethod{Name: "upload_save", Return: SQXReturnJSON, Handle: uploadSaveHandler},

		SQXMethod{Name: "request", Return: SQXReturnJSON, Handle: requestHandler},
		SQXMethod{Name: "parse_form", Return: SQXReturnJSON, Handle: parseFormHandler},
		SQXMethod{Name: "form_get", Return: SQXReturnString, Handle: formGetHandler},
		SQXMethod{Name: "parse_headers", Return: SQXReturnJSON, Handle: parseHeadersHandler},
		SQXMethod{Name: "header_get", Return: SQXReturnString, Handle: headerGetHandler},
		SQXMethod{Name: "cookie_get", Return: SQXReturnString, Handle: cookieGetHandler},
		SQXMethod{Name: "set_session_file", Return: SQXReturnString, Handle: setSessionFileHandler},
		SQXMethod{Name: "session_file", Return: SQXReturnString, Handle: sessionFileHandler},
		SQXMethod{Name: "set_session_cookie", Return: SQXReturnString, Handle: setSessionCookieHandler},
		SQXMethod{Name: "session_cookie", Return: SQXReturnString, Handle: sessionCookieHandler},
		SQXMethod{Name: "set_session_ttl", Return: SQXReturnInt, Handle: setSessionTTLHandler},
		SQXMethod{Name: "session_ttl", Return: SQXReturnInt, Handle: sessionTTLHandler},
		SQXMethod{Name: "set_secure_cookies", Return: SQXReturnBool, Handle: setSecureCookiesHandler},
		SQXMethod{Name: "secure_cookies", Return: SQXReturnBool, Handle: secureCookiesHandler},
		SQXMethod{Name: "password_hash", Return: SQXReturnString, Handle: passwordHashHandler},
		SQXMethod{Name: "password_verify", Return: SQXReturnInt, Handle: passwordVerifyHandler},
		SQXMethod{Name: "session_create", Return: SQXReturnJSON, Handle: sessionCreateHandler},
		SQXMethod{Name: "session_get", Return: SQXReturnJSON, Handle: sessionGetHandler},
		SQXMethod{Name: "session_destroy", Return: SQXReturnJSON, Handle: sessionDestroyHandler},
		SQXMethod{Name: "flash_set", Return: SQXReturnJSON, Handle: flashSetHandler},
		SQXMethod{Name: "flash_pop", Return: SQXReturnJSON, Handle: flashPopHandler},

		SQXMethod{Name: "db_open", Return: SQXReturnString, Handle: dbOpenHandler},
		SQXMethod{Name: "db_exec", Return: SQXReturnJSON, Handle: dbExecHandler},
		SQXMethod{Name: "db_query", Return: SQXReturnJSON, Handle: dbQueryHandler},
		SQXMethod{Name: "db_close", Return: SQXReturnString, Handle: dbCloseHandler},
		SQXMethod{Name: "db_path", Return: SQXReturnString, Handle: dbPathHandler},
	)

	os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}

func pingHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	return "http pong", nil
}

func getHandler(args []string) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("expected 1 or 2 arguments")
	}
	urlValue := strings.TrimSpace(args[0])
	if urlValue == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	req, err := http.NewRequest(http.MethodGet, urlValue, nil)
	if err != nil {
		return nil, err
	}
	if len(args) == 2 {
		headers, err := headersFromArg(args[1])
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  resp.StatusCode,
		"length":  len(body),
		"body":    string(body),
		"url":     urlValue,
		"headers": flattenHeader(resp.Header),
	}, nil
}

func postHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 4 {
		return nil, fmt.Errorf("expected 2 to 4 arguments")
	}

	urlValue := strings.TrimSpace(args[0])
	if urlValue == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	payloadRaw, err := SQXDecodeArg(args[1])
	if err != nil {
		return nil, err
	}
	payload, defaultContentType, err := normalizePayload(payloadRaw)
	if err != nil {
		return nil, err
	}

	contentType := defaultContentType
	if len(args) >= 3 {
		trimmed := strings.TrimSpace(args[2])
		if trimmed != "" {
			contentType = trimmed
		}
	}

	req, err := http.NewRequest(http.MethodPost, urlValue, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)

	if len(args) == 4 {
		headers, err := headersFromArg(args[3])
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  resp.StatusCode,
		"length":  len(body),
		"body":    string(body),
		"url":     urlValue,
		"headers": flattenHeader(resp.Header),
	}, nil
}

func statusHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	urlValue := strings.TrimSpace(args[0])
	if urlValue == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}
	resp, err := http.Head(urlValue)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func serverStartHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	port := strings.TrimSpace(args[0])
	if port == "" {
		return nil, fmt.Errorf("port cannot be empty")
	}

	state := loadServerState()
	if state.PID > 0 && isProcessRunning(state.PID) {
		return fmt.Sprintf("Server already running on port %s (PID: %d, logs: %s)", state.Port, state.PID, state.LogPath), nil
	}

	settings := loadSettings()
	if settings.StaticRoot == "" {
		settings.StaticRoot = resolvePath("static")
	}
	if settings.LogPath == "" {
		settings.LogPath = resolvePath("squ1dweb.log")
	}
	if err := saveSettings(settings); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(settings.LogPath), 0o755); err != nil {
		return nil, fmt.Errorf("could not create log directory: %w", err)
	}
	logHandle, err := os.OpenFile(settings.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("could not open log file: %w", err)
	}
	defer logHandle.Close()

	cmd := exec.Command(os.Args[0], "__server__", port)
	cmd.Stdout = logHandle
	cmd.Stderr = logHandle
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	state = ServerState{
		PID:        cmd.Process.Pid,
		Port:       port,
		LogPath:    settings.LogPath,
		StaticRoot: settings.StaticRoot,
		StartedAt:  time.Now().Format(time.RFC3339),
		TLS:        false,
	}
	if err := saveServerState(state); err != nil {
		return nil, err
	}

	return fmt.Sprintf("Server started on port %s (PID: %d, logs: %s)", port, state.PID, settings.LogPath), nil
}

func serverStartTLSHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 3); err != nil {
		return nil, err
	}
	port := strings.TrimSpace(args[0])
	if port == "" {
		return nil, fmt.Errorf("port cannot be empty")
	}
	certPath := resolvePath(args[1])
	keyPath := resolvePath(args[2])
	if !fileExists(certPath) {
		return nil, fmt.Errorf("certificate file not found: %s", certPath)
	}
	if !fileExists(keyPath) {
		return nil, fmt.Errorf("key file not found: %s", keyPath)
	}

	state := loadServerState()
	if state.PID > 0 && isProcessRunning(state.PID) {
		return fmt.Sprintf("Server already running on port %s (PID: %d, logs: %s)", state.Port, state.PID, state.LogPath), nil
	}

	settings := loadSettings()
	if settings.StaticRoot == "" {
		settings.StaticRoot = resolvePath("static")
	}
	if settings.LogPath == "" {
		settings.LogPath = resolvePath("squ1dweb.log")
	}
	if err := saveSettings(settings); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(settings.LogPath), 0o755); err != nil {
		return nil, fmt.Errorf("could not create log directory: %w", err)
	}
	logHandle, err := os.OpenFile(settings.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("could not open log file: %w", err)
	}
	defer logHandle.Close()

	cmd := exec.Command(os.Args[0], "__server_tls__", port, certPath, keyPath)
	cmd.Stdout = logHandle
	cmd.Stderr = logHandle
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start TLS server: %w", err)
	}

	state = ServerState{
		PID:        cmd.Process.Pid,
		Port:       port,
		LogPath:    settings.LogPath,
		StaticRoot: settings.StaticRoot,
		StartedAt:  time.Now().Format(time.RFC3339),
		TLS:        true,
		CertPath:   certPath,
		KeyPath:    keyPath,
	}
	if err := saveServerState(state); err != nil {
		return nil, err
	}

	return fmt.Sprintf("TLS server started on port %s (PID: %d, logs: %s)", port, state.PID, settings.LogPath), nil
}

func serverStopHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	state := loadServerState()
	if state.PID <= 0 {
		return "Server is not running", nil
	}

	proc, err := os.FindProcess(state.PID)
	if err != nil {
		clearServerState()
		return "Server was not running", nil
	}

	_ = proc.Kill()
	clearServerState()
	return fmt.Sprintf("Stopped server PID %d", state.PID), nil
}

func serverStatusHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	state := loadServerState()
	if state.PID <= 0 {
		return map[string]interface{}{"running": false}, nil
	}
	running := isProcessRunning(state.PID)
	return map[string]interface{}{
		"running":     running,
		"pid":         state.PID,
		"port":        state.Port,
		"log_path":    state.LogPath,
		"static_root": state.StaticRoot,
		"started_at":  state.StartedAt,
		"tls":         state.TLS,
		"cert_path":   state.CertPath,
		"key_path":    state.KeyPath,
	}, nil
}

func setRouteHandler(args []string) (interface{}, error) {
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("expected 3 or 4 arguments")
	}

	method := strings.ToUpper(strings.TrimSpace(args[0]))
	path := strings.TrimSpace(args[1])
	handler := strings.TrimSpace(args[2])
	action := ""
	if len(args) == 4 {
		action = strings.TrimSpace(args[3])
	}
	if method == "" || path == "" || handler == "" {
		return nil, fmt.Errorf("method/path/handler cannot be empty")
	}
	if !strings.EqualFold(handler, "static") && !filepath.IsAbs(handler) {
		handler = resolvePath(handler)
	}

	routeMutex.Lock()
	defer routeMutex.Unlock()

	cfg := loadRouteConfig()
	updated := false
	for i, r := range cfg.Routes {
		if strings.EqualFold(r.Method, method) && r.Path == path {
			cfg.Routes[i].Handler = handler
			cfg.Routes[i].Action = action
			updated = true
			break
		}
	}
	if !updated {
		cfg.Routes = append(cfg.Routes, RouteEntry{Method: method, Path: path, Handler: handler, Action: action})
	}
	sort.Slice(cfg.Routes, func(i, j int) bool {
		if cfg.Routes[i].Path == cfg.Routes[j].Path {
			return cfg.Routes[i].Method < cfg.Routes[j].Method
		}
		return cfg.Routes[i].Path < cfg.Routes[j].Path
	})

	if err := saveRouteConfig(cfg); err != nil {
		return nil, err
	}
	if action != "" {
		return fmt.Sprintf("Route %s %s -> %s#%s", method, path, handler, action), nil
	}
	return fmt.Sprintf("Route %s %s -> %s", method, path, handler), nil
}

func listRoutesHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	cfg := loadRouteConfig()
	return cfg.Routes, nil
}

func removeRouteHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 2); err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(args[0]))
	path := strings.TrimSpace(args[1])

	routeMutex.Lock()
	defer routeMutex.Unlock()

	cfg := loadRouteConfig()
	filtered := make([]RouteEntry, 0, len(cfg.Routes))
	for _, r := range cfg.Routes {
		if strings.EqualFold(r.Method, method) && r.Path == path {
			continue
		}
		filtered = append(filtered, r)
	}
	cfg.Routes = filtered
	if err := saveRouteConfig(cfg); err != nil {
		return nil, err
	}
	return fmt.Sprintf("Removed route %s %s", method, path), nil
}

func clearRoutesHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	routeMutex.Lock()
	defer routeMutex.Unlock()
	if err := saveRouteConfig(RouteConfig{}); err != nil {
		return nil, err
	}
	return "Cleared all routes", nil
}

func setStaticRootHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	root := strings.TrimSpace(args[0])
	if root == "" {
		return nil, fmt.Errorf("static root cannot be empty")
	}
	root = resolvePath(root)
	settings := loadSettings()
	settings.StaticRoot = root
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return root, nil
}

func staticRootHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.StaticRoot, nil
}

func setLogFileHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	path := strings.TrimSpace(args[0])
	if path == "" {
		return nil, fmt.Errorf("log path cannot be empty")
	}
	path = resolvePath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("could not create log directory: %w", err)
	}
	settings := loadSettings()
	settings.LogPath = path
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return path, nil
}

func logPathHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.LogPath, nil
}

func setUploadDirHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	dir := resolvePath(args[0])
	if dir == "" {
		return nil, fmt.Errorf("upload directory cannot be empty")
	}
	if !pathInsideProject(dir) {
		return nil, fmt.Errorf("upload directory must stay inside project")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	settings := loadSettings()
	settings.UploadDirectory = dir
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return dir, nil
}

func uploadDirHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.UploadDirectory, nil
}

func uploadUserDirHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	username := strings.TrimSpace(args[0])
	if username == "" {
		username = "user"
	}
	settings := loadSettings()
	base := settings.UploadDirectory
	if base == "" {
		base = resolvePath("uploads")
	}
	base = resolvePath(base)
	if !pathInsideProject(base) {
		return nil, fmt.Errorf("upload directory must stay inside project")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	slug := sanitizePathSegment(username)
	if slug == "" {
		slug = "user"
	}
	userDir := filepath.Join(base, slug)
	if !pathInsideDir(base, userDir) {
		return nil, fmt.Errorf("invalid user upload path")
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ok":   true,
		"slug": slug,
		"dir":  userDir,
	}, nil
}

func uploadPathHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	relative := strings.TrimSpace(args[0])
	if relative == "" {
		return nil, fmt.Errorf("upload path cannot be empty")
	}
	resolved, err := resolveUploadPath(relative)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func uploadDeleteHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	relative := strings.TrimSpace(args[0])
	if relative == "" {
		return nil, fmt.Errorf("upload path cannot be empty")
	}
	resolved, err := resolveUploadPath(relative)
	if err != nil {
		return nil, err
	}
	info, statErr := os.Stat(resolved)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return map[string]interface{}{"ok": false, "deleted": false, "reason": "not_found"}, nil
		}
		return nil, statErr
	}
	if info.IsDir() {
		return nil, fmt.Errorf("upload path points to directory")
	}
	if err := os.Remove(resolved); err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true, "deleted": true, "path": resolved}, nil
}

func logTailHandler(args []string) (interface{}, error) {
	lines := 40
	if len(args) > 1 {
		return nil, fmt.Errorf("expected 0 or 1 argument")
	}
	if len(args) == 1 {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil {
			return nil, fmt.Errorf("line count must be integer")
		}
		if parsed > 0 {
			lines = parsed
		}
	}

	settings := loadSettings()
	data, err := os.ReadFile(settings.LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return nil, err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	allLines := strings.Split(content, "\n")
	if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}
	if lines <= 0 || len(allLines) <= lines {
		return strings.Join(allLines, "\n"), nil
	}
	return strings.Join(allLines[len(allLines)-lines:], "\n"), nil
}

func redirectHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	status := parseStatus(args[0], http.StatusFound)
	location := strings.TrimSpace(args[1])
	if location == "" {
		return nil, fmt.Errorf("redirect location cannot be empty")
	}
	headers := map[string]string{"Location": location}
	if len(args) == 3 {
		extra, err := headersFromArg(args[2])
		if err != nil {
			return nil, err
		}
		for k, v := range extra {
			headers[k] = v
		}
	}
	return encodeRouteResponse(status, "text/plain; charset=utf-8", "", headers)
}

func responseHandler(args []string) (interface{}, error) {
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("expected 3 or 4 arguments")
	}
	status := parseStatus(args[0], http.StatusOK)
	contentType := strings.TrimSpace(args[1])
	if contentType == "" {
		contentType = "text/plain"
	}
	body, err := SQXDecodeArg(args[2])
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if len(args) == 4 {
		headers, err = headersFromArg(args[3])
		if err != nil {
			return nil, err
		}
	}
	return encodeRouteResponse(status, contentType, body, headers)
}

func responseJSONHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	status := parseStatus(args[0], http.StatusOK)
	body, err := SQXDecodeArg(args[1])
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if len(args) == 3 {
		headers, err = headersFromArg(args[2])
		if err != nil {
			return nil, err
		}
	}
	return encodeRouteResponse(status, "application/json", body, headers)
}

func responseHTMLHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	status := parseStatus(args[0], http.StatusOK)
	html, err := SQXArgString(args, 1)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if len(args) == 3 {
		headers, err = headersFromArg(args[2])
		if err != nil {
			return nil, err
		}
	}
	return encodeRouteResponse(status, "text/html; charset=utf-8", html, headers)
}

func responseTextHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	status := parseStatus(args[0], http.StatusOK)
	text, err := SQXArgString(args, 1)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if len(args) == 3 {
		headers, err = headersFromArg(args[2])
		if err != nil {
			return nil, err
		}
	}
	return encodeRouteResponse(status, "text/plain; charset=utf-8", text, headers)
}

func renderFileHandler(args []string) (interface{}, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("expected 1 to 3 arguments")
	}

	status := http.StatusOK
	pathIndex := 0
	if len(args) >= 2 {
		if _, err := strconv.Atoi(strings.TrimSpace(args[0])); err == nil {
			status = parseStatus(args[0], http.StatusOK)
			pathIndex = 1
		}
	}

	filePath, err := SQXArgString(args, pathIndex)
	if err != nil {
		return nil, err
	}
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	if !filepath.IsAbs(filePath) {
		filePath = resolvePath(filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	contentType := ""
	if len(args) > pathIndex+1 {
		contentType = strings.TrimSpace(args[pathIndex+1])
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filePath))
	}
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}

	return encodeRouteResponse(status, contentType, string(content), map[string]string{})
}

func renderTemplateHandler(args []string) (interface{}, error) {
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("expected 3 or 4 arguments")
	}
	status := parseStatus(args[0], http.StatusOK)
	templatePath, err := SQXArgString(args, 1)
	if err != nil {
		return nil, err
	}
	if templatePath == "" {
		return nil, fmt.Errorf("template path cannot be empty")
	}
	if !filepath.IsAbs(templatePath) {
		templatePath = resolvePath(templatePath)
	}
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}
	data, err := SQXDecodeArg(args[2])
	if err != nil {
		return nil, err
	}
	rendered := renderTemplate(string(content), data)
	headers := map[string]string{}
	if len(args) == 4 {
		headers, err = headersFromArg(args[3])
		if err != nil {
			return nil, err
		}
	}
	return encodeRouteResponse(status, "text/html; charset=utf-8", rendered, headers)
}

func escapeHTMLHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	value, err := SQXArgString(args, 0)
	if err != nil {
		return nil, err
	}
	return html.EscapeString(value), nil
}

func downloadFileHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	status := parseStatus(args[0], http.StatusOK)
	filePath, err := SQXArgString(args, 1)
	if err != nil {
		return nil, err
	}
	filePath = resolvePath(filePath)
	if !pathInsideProject(filePath) {
		return nil, fmt.Errorf("download path must stay inside project")
	}
	if !fileExists(filePath) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	downloadName := sanitizeFileName(filepath.Base(filePath))
	if len(args) == 3 {
		name := strings.TrimSpace(args[2])
		if name != "" {
			downloadName = sanitizeFileName(name)
		}
	}
	if downloadName == "" {
		downloadName = "download.bin"
	}
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	headers := map[string]string{
		"Content-Disposition": fmt.Sprintf("attachment; filename=\"%s\"", downloadName),
	}
	return encodeRouteResponse(status, contentType, string(content), headers)
}

func uploadSaveHandler(args []string) (interface{}, error) {
	if len(args) < 4 || len(args) > 5 {
		return nil, fmt.Errorf("expected 4 or 5 arguments")
	}
	bodyFile := resolvePath(strings.TrimSpace(args[0]))
	contentType := strings.TrimSpace(args[1])
	fieldName := strings.TrimSpace(args[2])
	outDir := resolvePath(strings.TrimSpace(args[3]))
	if bodyFile == "" || !fileExists(bodyFile) {
		return nil, fmt.Errorf("body file not found")
	}
	if contentType == "" {
		return nil, fmt.Errorf("content type cannot be empty")
	}
	if fieldName == "" {
		fieldName = "file"
	}
	if outDir == "" {
		settings := loadSettings()
		outDir = settings.UploadDirectory
		if outDir == "" {
			outDir = resolvePath("uploads")
		}
	}
	if !pathInsideProject(outDir) {
		return nil, fmt.Errorf("upload directory must stay inside project")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	body, err := os.ReadFile(bodyFile)
	if err != nil {
		return nil, err
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("invalid multipart content type: %w", err)
	}
	if !strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return nil, fmt.Errorf("content type must be multipart/*")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("multipart boundary missing")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() != fieldName || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		original := sanitizeFileName(part.FileName())
		if original == "" {
			original = "upload.bin"
		}
		random, err := randomToken(6)
		if err != nil {
			return nil, err
		}
		stored := random + "_" + original
		target := filepath.Join(outDir, stored)
		file, err := os.Create(target)
		if err != nil {
			return nil, err
		}
		size, copyErr := io.Copy(file, part)
		closeErr := file.Close()
		_ = part.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return map[string]interface{}{
			"ok":            true,
			"field":         fieldName,
			"original_name": original,
			"stored_name":   stored,
			"path":          target,
			"size":          size,
		}, nil
	}

	return nil, fmt.Errorf("multipart field %q not found", fieldName)
}

func requestHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	ctx, ok := loadCurrentRequestContext()
	if !ok {
		return map[string]interface{}{
			"method":       "",
			"path":         "",
			"route":        "",
			"action":       "",
			"query":        "",
			"content_type": "",
			"cookie":       "",
			"body":         "",
			"body_file":    "",
			"headers":      map[string]string{},
			"headers_json": "{}",
		}, nil
	}
	if ctx.Headers == nil {
		ctx.Headers = map[string]string{}
	}
	headersJSON, err := json.Marshal(ctx.Headers)
	if err != nil {
		headersJSON = []byte("{}")
	}
	return map[string]interface{}{
		"method":       ctx.Method,
		"path":         ctx.Path,
		"route":        ctx.Route,
		"action":       ctx.Action,
		"query":        ctx.Query,
		"content_type": ctx.ContentType,
		"cookie":       ctx.Cookie,
		"body":         ctx.Body,
		"body_file":    ctx.BodyFile,
		"headers":      ctx.Headers,
		"headers_json": string(headersJSON),
	}, nil
}

func parseFormHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	values, err := url.ParseQuery(args[0])
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{}, len(values))
	for k, v := range values {
		if len(v) == 0 {
			result[k] = ""
			continue
		}
		result[k] = v[0]
	}
	return result, nil
}

func formGetHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	values, err := url.ParseQuery(args[0])
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(args[1])
	if key == "" {
		return "", nil
	}
	if val := values.Get(key); val != "" {
		return val, nil
	}
	if len(args) == 3 {
		return args[2], nil
	}
	return "", nil
}

func parseHeadersHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	return parseHeaderJSON(args[0])
}

func headerGetHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	headers, err := parseHeaderJSON(args[0])
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(args[1])
	if key == "" {
		if len(args) == 3 {
			return args[2], nil
		}
		return "", nil
	}

	for hk, hv := range headers {
		if strings.EqualFold(hk, key) {
			return hv, nil
		}
	}

	if len(args) == 3 {
		return args[2], nil
	}
	return "", nil
}

func cookieGetHandler(args []string) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("expected 2 or 3 arguments")
	}
	headers, err := headersFromHeaderArg(args[0])
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(args[1])
	if name == "" {
		return "", nil
	}
	value := cookieFromHeaders(headers, name)
	if value != "" {
		return value, nil
	}
	if len(args) == 3 {
		return args[2], nil
	}
	return "", nil
}

func setSessionFileHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	path := resolvePath(args[0])
	if path == "" {
		return nil, fmt.Errorf("session file path cannot be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	settings := loadSettings()
	settings.SessionPath = path
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return path, nil
}

func sessionFileHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.SessionPath, nil
}

func setSessionCookieHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(args[0])
	if name == "" {
		return nil, fmt.Errorf("session cookie cannot be empty")
	}
	settings := loadSettings()
	settings.SessionCookie = name
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return name, nil
}

func sessionCookieHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.SessionCookie, nil
}

func setSessionTTLHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	ttl, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || ttl <= 0 {
		return nil, fmt.Errorf("session ttl must be a positive integer")
	}
	settings := loadSettings()
	settings.SessionTTL = ttl
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return int64(ttl), nil
}

func sessionTTLHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return int64(settings.SessionTTL), nil
}

func setSecureCookiesHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	value, err := SQXArgBool(args, 0)
	if err != nil {
		return nil, err
	}
	settings := loadSettings()
	settings.SecureCookies = value
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return value, nil
}

func secureCookiesHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.SecureCookies, nil
}

func passwordHashHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	password := strings.TrimSpace(args[0])
	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	return hash, nil
}

func passwordVerifyHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 2); err != nil {
		return nil, err
	}
	password := args[0]
	encoded := strings.TrimSpace(args[1])
	if verifyPassword(password, encoded) {
		return int64(1), nil
	}
	return int64(0), nil
}

func sessionCreateHandler(args []string) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("expected 1 or 2 arguments")
	}
	settings := loadSettings()
	user, err := SQXDecodeArg(args[0])
	if err != nil {
		return nil, err
	}
	ttl := settings.SessionTTL
	if len(args) == 2 {
		parsed, convErr := strconv.Atoi(strings.TrimSpace(args[1]))
		if convErr != nil || parsed <= 0 {
			return nil, fmt.Errorf("session ttl must be a positive integer")
		}
		ttl = parsed
	}
	if ttl <= 0 {
		ttl = 86400
	}
	id, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	exp := now.Add(time.Duration(ttl) * time.Second)
	record := SessionRecord{
		ID:        id,
		User:      user,
		CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: exp.Format(time.RFC3339),
	}

	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	store := loadSessionStore(settings.SessionPath)
	cleanupExpiredSessions(&store)
	store.Sessions[id] = record
	if err := saveSessionStore(settings.SessionPath, store); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ok":         true,
		"id":         id,
		"created_at": record.CreatedAt,
		"expires_at": record.ExpiresAt,
		"set_cookie": buildSessionCookie(settings, id, ttl, false),
	}, nil
}

func sessionGetHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	settings := loadSettings()
	headers, err := headersFromHeaderArg(args[0])
	if err != nil {
		return nil, err
	}
	sessionID := cookieFromHeaders(headers, settings.SessionCookie)
	if sessionID == "" {
		return map[string]interface{}{"ok": false, "id": "", "user": nil}, nil
	}

	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	store := loadSessionStore(settings.SessionPath)
	cleanupExpiredSessions(&store)
	record, ok := store.Sessions[sessionID]
	if !ok {
		_ = saveSessionStore(settings.SessionPath, store)
		return map[string]interface{}{"ok": false, "id": "", "user": nil}, nil
	}
	_ = saveSessionStore(settings.SessionPath, store)
	return map[string]interface{}{
		"ok":         true,
		"id":         record.ID,
		"user":       record.User,
		"created_at": record.CreatedAt,
		"expires_at": record.ExpiresAt,
	}, nil
}

func sessionDestroyHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	settings := loadSettings()
	headers, err := headersFromHeaderArg(args[0])
	if err != nil {
		return nil, err
	}
	sessionID := cookieFromHeaders(headers, settings.SessionCookie)

	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	store := loadSessionStore(settings.SessionPath)
	if sessionID != "" {
		delete(store.Sessions, sessionID)
	}
	cleanupExpiredSessions(&store)
	if err := saveSessionStore(settings.SessionPath, store); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ok":         true,
		"set_cookie": buildSessionCookie(settings, "", 0, true),
	}, nil
}

func flashSetHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 2); err != nil {
		return nil, err
	}
	settings := loadSettings()
	headers, err := headersFromHeaderArg(args[0])
	if err != nil {
		return nil, err
	}
	sessionID := cookieFromHeaders(headers, settings.SessionCookie)
	if sessionID == "" {
		return map[string]interface{}{"ok": false, "reason": "no_session"}, nil
	}
	flashData, err := SQXDecodeArg(args[1])
	if err != nil {
		return nil, err
	}

	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	store := loadSessionStore(settings.SessionPath)
	cleanupExpiredSessions(&store)
	record, ok := store.Sessions[sessionID]
	if !ok {
		_ = saveSessionStore(settings.SessionPath, store)
		return map[string]interface{}{"ok": false, "reason": "not_found"}, nil
	}
	record.Flash = flashData
	store.Sessions[sessionID] = record
	if err := saveSessionStore(settings.SessionPath, store); err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true}, nil
}

func flashPopHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	settings := loadSettings()
	headers, err := headersFromHeaderArg(args[0])
	if err != nil {
		return nil, err
	}
	sessionID := cookieFromHeaders(headers, settings.SessionCookie)
	if sessionID == "" {
		return map[string]interface{}{}, nil
	}

	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	store := loadSessionStore(settings.SessionPath)
	cleanupExpiredSessions(&store)
	record, ok := store.Sessions[sessionID]
	if !ok {
		_ = saveSessionStore(settings.SessionPath, store)
		return map[string]interface{}{}, nil
	}
	flash := record.Flash
	record.Flash = nil
	store.Sessions[sessionID] = record
	if err := saveSessionStore(settings.SessionPath, store); err != nil {
		return nil, err
	}
	if flash == nil {
		return map[string]interface{}{}, nil
	}
	return flash, nil
}

func dbOpenHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	path := strings.TrimSpace(args[0])
	if path == "" {
		return nil, fmt.Errorf("db path cannot be empty")
	}
	path = resolvePath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	_ = db.Close()

	settings := loadSettings()
	settings.DBPath = path
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return fmt.Sprintf("DB opened %s", path), nil
}

func dbExecHandler(args []string) (interface{}, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("expected 1 to 3 arguments")
	}
	query := strings.TrimSpace(args[0])
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	params, err := decodeParamsArg(args, 1)
	if err != nil {
		return nil, err
	}
	path, err := resolveDBPath(args, 2)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	res, err := db.Exec(query, params...)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	return map[string]interface{}{"rows_affected": rows, "last_insert_id": lastID, "db_path": path}, nil
}

func dbQueryHandler(args []string) (interface{}, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("expected 1 to 3 arguments")
	}
	query := strings.TrimSpace(args[0])
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	params, err := decodeParamsArg(args, 1)
	if err != nil {
		return nil, err
	}
	path, err := resolveDBPath(args, 2)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			rowMap[col] = normalizeDBValue(values[i])
		}
		results = append(results, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func dbCloseHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	settings.DBPath = ""
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return "DB path cleared", nil
}

func dbPathHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	settings := loadSettings()
	return settings.DBPath, nil
}

func loadRouteConfig() RouteConfig {
	data, err := os.ReadFile(routeConfigPath)
	if err != nil {
		return RouteConfig{}
	}
	var cfg RouteConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return RouteConfig{}
	}
	return cfg
}

func saveRouteConfig(cfg RouteConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(routeConfigPath, data, 0o644); err != nil {
		return err
	}
	return nil
}

func startWebServer(port string) {
	settings := loadSettings()
	staticRoot = settings.StaticRoot
	logFilePath = settings.LogPath

	mux := http.NewServeMux()
	mux.HandleFunc("/", genericHandler)

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logWebEvent("Listen error: " + err.Error())
		return
	}
	defer ln.Close()

	logWebEvent("Server listening on :" + port)
	srv := &http.Server{Handler: loggingMiddleware(mux)}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		logWebEvent("Server error: " + err.Error())
	}
}

func startWebServerTLS(port, certPath, keyPath string) {
	settings := loadSettings()
	staticRoot = settings.StaticRoot
	logFilePath = settings.LogPath

	mux := http.NewServeMux()
	mux.HandleFunc("/", genericHandler)
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: loggingMiddleware(mux),
	}

	logWebEvent("TLS server listening on :" + port)
	if err := srv.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
		logWebEvent("TLS server error: " + err.Error())
	}
}

func genericHandler(w http.ResponseWriter, r *http.Request) {
	cfg := loadRouteConfig()
	path := r.URL.Path
	method := strings.ToUpper(r.Method)
	matched := findRoute(cfg.Routes, method, path)
	if matched == nil {
		http.NotFound(w, r)
		return
	}

	if strings.EqualFold(matched.Handler, "static") || strings.HasPrefix(matched.Path, "/static/") {
		serveStaticFile(w, r)
		return
	}

	body, _ := io.ReadAll(r.Body)
	resp, err := executeRouteHandler(*matched, r, body)
	if err != nil {
		logWebEvent("Route handler error: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	contentType := resp.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}
	if _, isObj := resp.Body.(map[string]interface{}); isObj && contentType == "text/plain" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.Status)

	switch b := resp.Body.(type) {
	case string:
		_, _ = w.Write([]byte(b))
	case []byte:
		_, _ = w.Write(b)
	default:
		if out, err := json.Marshal(b); err == nil {
			_, _ = w.Write(out)
		} else {
			_, _ = w.Write([]byte(fmt.Sprintf("%v", b)))
		}
	}
}

func findRoute(routes []RouteEntry, method, path string) *RouteEntry {
	var selected *RouteEntry
	bestPrefixLen := -1
	for i, route := range routes {
		if !strings.EqualFold(route.Method, method) {
			continue
		}
		if route.Path == path {
			return &routes[i]
		}
		if strings.HasSuffix(route.Path, "/*") {
			prefix := strings.TrimSuffix(route.Path, "/*")
			if strings.HasPrefix(path, prefix) && len(prefix) > bestPrefixLen {
				selected = &routes[i]
				bestPrefixLen = len(prefix)
			}
		}
		if strings.HasSuffix(route.Path, "/") && strings.HasPrefix(path, route.Path) && len(route.Path) > bestPrefixLen {
			selected = &routes[i]
			bestPrefixLen = len(route.Path)
		}
	}
	return selected
}

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	root := staticRoot
	if root == "" {
		settings := loadSettings()
		root = settings.StaticRoot
	}
	if root == "" {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/static/")
	if trimmed == "" {
		trimmed = "index.html"
	}

	cleaned := filepath.Clean(trimmed)
	if strings.HasPrefix(cleaned, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	baseAbs, err := filepath.Abs(root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fname := filepath.Join(baseAbs, cleaned)
	fnameAbs, err := filepath.Abs(fname)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(fnameAbs, baseAbs) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if info, err := os.Stat(fnameAbs); err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fnameAbs)
}

func executeRouteHandler(route RouteEntry, r *http.Request, requestBody []byte) (*RouteResponse, error) {
	handlerPath := route.Handler
	if handlerPath == "" {
		return nil, fmt.Errorf("empty handler path")
	}

	absHandlerPath := handlerPath
	if !filepath.IsAbs(absHandlerPath) {
		absHandlerPath = resolvePath(handlerPath)
	}

	headers := flattenHeader(r.Header)
	cookieHeader := r.Header.Get("Cookie")
	if cookieHeader == "" {
		cookies := r.Cookies()
		if len(cookies) > 0 {
			pairs := make([]string, 0, len(cookies))
			for _, cookie := range cookies {
				pairs = append(pairs, cookie.Name+"="+cookie.Value)
			}
			cookieHeader = strings.Join(pairs, "; ")
		}
	}
	bodyFile := ""
	if len(requestBody) > 0 {
		tmpDir := stateDirPath
		if tmpDir == "" {
			tmpDir = "."
		}
		tmpFile, err := os.CreateTemp(tmpDir, "squ1dweb_body_*")
		if err == nil {
			if _, writeErr := tmpFile.Write(requestBody); writeErr == nil {
				bodyFile = tmpFile.Name()
			}
			_ = tmpFile.Close()
			if bodyFile == "" {
				_ = os.Remove(tmpFile.Name())
			}
		}
	}
	if bodyFile != "" {
		defer os.Remove(bodyFile)
	}

	ctx := RequestContext{
		Method:      r.Method,
		Path:        r.URL.Path,
		Route:       route.Path,
		Action:      route.Action,
		Query:       r.URL.RawQuery,
		ContentType: r.Header.Get("Content-Type"),
		Cookie:      cookieHeader,
		Body:        requestBodyToText(requestBody),
		BodyFile:    bodyFile,
		Headers:     headers,
	}

	cmd := exec.Command(squ1dccPath, absHandlerPath)
	cmd.Dir = findProjectRoot(absHandlerPath)
	cmd.Stdin = bytes.NewReader(requestBody)
	cmd.Env = os.Environ()
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("route handler %s start error: %w", absHandlerPath, err)
	}
	routePID := cmd.Process.Pid
	if routePID > 0 {
		if err := writeRequestContext(routePID, ctx); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("route handler %s context error: %w", absHandlerPath, err)
		}
		defer removeRequestContext(routePID)
	}

	err := cmd.Wait()
	out := output.Bytes()
	if err != nil {
		return nil, fmt.Errorf("route handler %s error: %v output=%s", absHandlerPath, err, strings.TrimSpace(string(out)))
	}

	var routeResp RouteResponse
	if err := json.Unmarshal(out, &routeResp); err == nil {
		if routeResp.Status == 0 {
			routeResp.Status = http.StatusOK
		}
		if routeResp.Headers == nil {
			routeResp.Headers = map[string]string{}
		}
		return &routeResp, nil
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(out, &generic); err == nil {
		resp := &RouteResponse{
			Status:      http.StatusOK,
			ContentType: "text/plain",
			Body:        "",
			Headers:     map[string]string{},
		}
		if body, ok := generic["body"]; ok {
			resp.Body = body
		}
		if statusNum, ok := generic["status"].(float64); ok {
			resp.Status = int(statusNum)
		}
		if contentType, ok := generic["content_type"].(string); ok && strings.TrimSpace(contentType) != "" {
			resp.ContentType = contentType
		}
		if headers, ok := generic["headers"].(map[string]interface{}); ok {
			for k, v := range headers {
				resp.Headers[k] = fmt.Sprint(v)
			}
		}
		return resp, nil
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return &RouteResponse{Status: http.StatusNoContent, ContentType: "text/plain", Body: "", Headers: map[string]string{}}, nil
	}

	return &RouteResponse{Status: http.StatusOK, ContentType: "text/plain", Body: string(out), Headers: map[string]string{}}, nil
}

type loggingResponseWriter struct {
	w       http.ResponseWriter
	status  int
	written int
}

func (lrw *loggingResponseWriter) Header() http.Header { return lrw.w.Header() }

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.status == 0 {
		lrw.status = http.StatusOK
	}
	n, err := lrw.w.Write(b)
	lrw.written += n
	return n, err
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.status = statusCode
	lrw.w.WriteHeader(statusCode)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{w: w}
		start := time.Now()
		next.ServeHTTP(lrw, r)
		if lrw.status == 0 {
			lrw.status = http.StatusOK
		}
		logWebEvent(fmt.Sprintf("%s %s %s %d %d %v", r.RemoteAddr, r.Method, r.URL.Path, lrw.status, lrw.written, time.Since(start)))
	})
}

func logWebEvent(msg string) {
	logMutex.Lock()
	defer logMutex.Unlock()
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	_, _ = os.Stderr.WriteString(line)
}

func loadSettings() EngineSettings {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	defaults := EngineSettings{
		StaticRoot:      resolvePath("static"),
		LogPath:         resolvePath("squ1dweb.log"),
		DBPath:          resolvePath("squ1dweb.db"),
		SessionPath:     resolvePath("squ1dweb_sessions.json"),
		SessionCookie:   "squ1d_session",
		SessionTTL:      86400,
		SecureCookies:   false,
		UploadDirectory: resolvePath("uploads"),
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return defaults
	}
	var settings EngineSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaults
	}
	if settings.StaticRoot == "" {
		settings.StaticRoot = defaults.StaticRoot
	}
	if settings.LogPath == "" {
		settings.LogPath = defaults.LogPath
	}
	if settings.DBPath == "" {
		settings.DBPath = defaults.DBPath
	}
	if settings.SessionPath == "" {
		settings.SessionPath = defaults.SessionPath
	}
	if settings.SessionCookie == "" {
		settings.SessionCookie = defaults.SessionCookie
	}
	if settings.SessionTTL <= 0 {
		settings.SessionTTL = defaults.SessionTTL
	}
	if settings.UploadDirectory == "" {
		settings.UploadDirectory = defaults.UploadDirectory
	}
	settings.StaticRoot = resolvePath(settings.StaticRoot)
	settings.LogPath = resolvePath(settings.LogPath)
	settings.DBPath = resolvePath(settings.DBPath)
	settings.SessionPath = resolvePath(settings.SessionPath)
	settings.UploadDirectory = resolvePath(settings.UploadDirectory)
	return settings
}

func saveSettings(settings EngineSettings) error {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	settings.StaticRoot = resolvePath(settings.StaticRoot)
	settings.LogPath = resolvePath(settings.LogPath)
	settings.DBPath = resolvePath(settings.DBPath)
	settings.SessionPath = resolvePath(settings.SessionPath)
	settings.UploadDirectory = resolvePath(settings.UploadDirectory)
	if settings.SessionCookie == "" {
		settings.SessionCookie = "squ1d_session"
	}
	if settings.SessionTTL <= 0 {
		settings.SessionTTL = 86400
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, data, 0o644)
}

func loadServerState() ServerState {
	data, err := os.ReadFile(serverStatePath)
	if err != nil {
		return ServerState{}
	}
	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return ServerState{}
	}
	return state
}

func saveServerState(state ServerState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(serverStatePath, data, 0o644)
}

func clearServerState() {
	_ = os.Remove(serverStatePath)
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func flattenHeader(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = strings.Join(v, ",")
	}
	return out
}

func requestBodyToText(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if bytes.IndexByte(body, 0) >= 0 {
		return ""
	}
	if !utf8.Valid(body) {
		return ""
	}
	return string(body)
}

func requestContextPath(pid int) string {
	if requestCtxDir == "" {
		requestCtxDir = filepath.Join(stateDirPath, "request_ctx")
	}
	return filepath.Join(requestCtxDir, fmt.Sprintf("%d.json", pid))
}

func writeRequestContext(pid int, ctx RequestContext) error {
	if pid <= 0 {
		return nil
	}
	if requestCtxDir == "" {
		requestCtxDir = filepath.Join(stateDirPath, "request_ctx")
	}
	if err := os.MkdirAll(requestCtxDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile(requestContextPath(pid), data, 0o600)
}

func removeRequestContext(pid int) {
	if pid <= 0 {
		return
	}
	_ = os.Remove(requestContextPath(pid))
}

func loadRequestContext(pid int) (RequestContext, error) {
	ctx := RequestContext{}
	if pid <= 0 {
		return ctx, fmt.Errorf("invalid pid")
	}
	data, err := os.ReadFile(requestContextPath(pid))
	if err != nil {
		return ctx, err
	}
	if err := json.Unmarshal(data, &ctx); err != nil {
		return ctx, err
	}
	if ctx.Headers == nil {
		ctx.Headers = map[string]string{}
	}
	return ctx, nil
}

func loadCurrentRequestContext() (RequestContext, bool) {
	ownerPID := os.Getppid()
	for i := 0; i < 40; i++ {
		ctx, err := loadRequestContext(ownerPID)
		if err == nil {
			return ctx, true
		}
		if !os.IsNotExist(err) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return RequestContext{}, false
}

func parseHeaderJSON(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}, nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("invalid headers JSON: %w", err)
	}
	out := make(map[string]string, len(parsed))
	for k, v := range parsed {
		out[k] = fmt.Sprint(v)
	}
	return out, nil
}

func headersFromHeaderArg(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return parseHeaderJSON(trimmed)
	}
	return map[string]string{"Cookie": trimmed}, nil
}

func cookieFromHeaders(headers map[string]string, name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	cookieHeader := ""
	for hk, hv := range headers {
		if strings.EqualFold(hk, "Cookie") {
			cookieHeader = hv
			break
		}
	}
	if cookieHeader == "" {
		return ""
	}
	for _, chunk := range strings.Split(cookieHeader, ";") {
		part := strings.TrimSpace(chunk)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(pieces[0]), name) {
			return strings.TrimSpace(pieces[1])
		}
	}
	return ""
}

func headersFromArg(raw string) (map[string]string, error) {
	decoded, err := SQXDecodeArg(raw)
	if err != nil {
		return nil, err
	}
	if decoded == nil {
		return map[string]string{}, nil
	}
	obj, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("headers must be an object")
	}
	headers := make(map[string]string, len(obj))
	for k, v := range obj {
		headers[k] = fmt.Sprint(v)
	}
	return headers, nil
}

func normalizePayload(value interface{}) (string, string, error) {
	switch v := value.(type) {
	case nil:
		return "", "application/json", nil
	case string:
		return v, "application/json", nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", "", err
		}
		return string(data), "application/json", nil
	}
}

func parseStatus(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < 100 || parsed > 999 {
		return fallback
	}
	return parsed
}

func encodeRouteResponse(status int, contentType string, body interface{}, headers map[string]string) (string, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	resp := RouteResponse{
		Status:      status,
		ContentType: contentType,
		Body:        body,
		Headers:     headers,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderTemplate(content string, data interface{}) string {
	withRaw := templateRawExprPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatch := templateRawExprPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return ""
		}
		value := resolveTemplateValue(data, submatch[1])
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	})
	return templateExprPattern.ReplaceAllStringFunc(withRaw, func(match string) string {
		submatch := templateExprPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return ""
		}
		value := resolveTemplateValue(data, submatch[1])
		if value == nil {
			return ""
		}
		return html.EscapeString(fmt.Sprint(value))
	})
}

func resolveTemplateValue(data interface{}, key string) interface{} {
	if key == "" {
		return nil
	}
	current := data
	parts := strings.Split(key, ".")
	for _, part := range parts {
		switch node := current.(type) {
		case map[string]interface{}:
			val, ok := node[part]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil
		}
	}
	return current
}

func randomToken(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "._")
}

func loadSessionStore(path string) SessionStore {
	store := SessionStore{Sessions: map[string]SessionRecord{}}
	if path == "" {
		return store
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return store
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return SessionStore{Sessions: map[string]SessionRecord{}}
	}
	if store.Sessions == nil {
		store.Sessions = map[string]SessionRecord{}
	}
	return store
}

func saveSessionStore(path string, store SessionStore) error {
	if path == "" {
		return fmt.Errorf("session file path is empty")
	}
	if store.Sessions == nil {
		store.Sessions = map[string]SessionRecord{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func cleanupExpiredSessions(store *SessionStore) {
	if store == nil || len(store.Sessions) == 0 {
		return
	}
	now := time.Now().UTC()
	for id, rec := range store.Sessions {
		exp, err := time.Parse(time.RFC3339, rec.ExpiresAt)
		if err != nil || !exp.After(now) {
			delete(store.Sessions, id)
		}
	}
}

func buildSessionCookie(settings EngineSettings, token string, ttl int, clear bool) string {
	name := settings.SessionCookie
	if name == "" {
		name = "squ1d_session"
	}
	base := fmt.Sprintf("%s=%s; Path=/; HttpOnly; SameSite=Lax", name, token)
	if settings.SecureCookies {
		base += "; Secure"
	}
	if clear {
		return base + "; Max-Age=0"
	}
	if ttl <= 0 {
		ttl = settings.SessionTTL
	}
	if ttl <= 0 {
		ttl = 86400
	}
	return fmt.Sprintf("%s; Max-Age=%d", base, ttl)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := derivePasswordHash(password, salt)
	return fmt.Sprintf("sqsha256$v1$%s$%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false
	}
	if parts[0] != "sqsha256" || parts[1] != "v1" {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := derivePasswordHash(password, salt)
	if len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

func derivePasswordHash(password string, salt []byte) []byte {
	payload := make([]byte, 0, len(salt)+len(password))
	payload = append(payload, salt...)
	payload = append(payload, []byte(password)...)
	sum := sha256.Sum256(payload)
	digest := sum[:]

	// Keep this reasonably expensive for brute-force resistance while staying
	// lightweight enough for example usage.
	const rounds = 120000
	for i := 0; i < rounds; i++ {
		mix := make([]byte, 0, len(salt)+len(digest))
		mix = append(mix, salt...)
		mix = append(mix, digest...)
		step := sha256.Sum256(mix)
		digest = step[:]
	}

	out := make([]byte, len(digest))
	copy(out, digest)
	return out
}

func resolvePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return trimmed
		}
		projectRoot = cwd
	}
	return filepath.Clean(filepath.Join(projectRoot, trimmed))
}

func decodeParamsArg(args []string, index int) ([]interface{}, error) {
	if len(args) <= index {
		return []interface{}{}, nil
	}
	decoded, err := SQXDecodeArg(args[index])
	if err != nil {
		return nil, err
	}
	switch v := decoded.(type) {
	case nil:
		return []interface{}{}, nil
	case []interface{}:
		return v, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []interface{}{}, nil
		}
		var parsed []interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed, nil
		}
		return []interface{}{v}, nil
	default:
		return []interface{}{v}, nil
	}
}

func resolveDBPath(args []string, index int) (string, error) {
	if len(args) > index {
		override := strings.TrimSpace(args[index])
		if override != "" {
			return resolvePath(override), nil
		}
	}
	settings := loadSettings()
	if settings.DBPath == "" {
		return "", fmt.Errorf("database path is empty")
	}
	return settings.DBPath, nil
}

func normalizeDBValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}

func findProjectRoot(handlerPath string) string {
	current := filepath.Dir(handlerPath)
	for {
		libDir := filepath.Join(current, "lib")
		if info, err := os.Stat(libDir); err == nil && info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Dir(handlerPath)
		}
		current = parent
	}
}

func detectProjectRoot() string {
	if fromEnv := strings.TrimSpace(os.Getenv("SQX_PROJECT_ROOT")); fromEnv != "" {
		return filepath.Clean(fromEnv)
	}
	if fromEnv := strings.TrimSpace(os.Getenv("SQU1DWEB_ROOT")); fromEnv != "" {
		return filepath.Clean(fromEnv)
	}

	start := ""
	cwd, err := os.Getwd()
	if err == nil {
		start = cwd
	}
	if start == "" {
		exe, err := os.Executable()
		if err == nil {
			start = filepath.Dir(exe)
		}
	}
	if start == "" {
		return "."
	}

	current := start
	for {
		mainPath := filepath.Join(current, "main.sqd")
		routesPath := filepath.Join(current, "routes")
		staticPath := filepath.Join(current, "static")
		if fileExists(mainPath) || (dirExists(routesPath) && dirExists(staticPath)) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return start
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathInsideProject(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	root := projectRoot
	if root == "" {
		root = detectProjectRoot()
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if absPath == absRoot {
		return true
	}
	return strings.HasPrefix(absPath, absRoot+string(os.PathSeparator))
}

func pathInsideDir(baseDir, target string) bool {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(target) == "" {
		return false
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	if absTarget == absBase {
		return true
	}
	return strings.HasPrefix(absTarget, absBase+string(os.PathSeparator))
}

func sanitizePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	if out == "" {
		return ""
	}
	return out
}

func resolveUploadPath(relative string) (string, error) {
	settings := loadSettings()
	base := settings.UploadDirectory
	if base == "" {
		base = resolvePath("uploads")
	}
	base = resolvePath(base)
	if !pathInsideProject(base) {
		return "", fmt.Errorf("upload directory must stay inside project")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	clean := filepath.Clean(strings.TrimSpace(relative))
	if clean == "" || clean == "." {
		return "", fmt.Errorf("upload path cannot be empty")
	}
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("upload path escapes upload directory")
	}
	target := filepath.Join(base, clean)
	if !pathInsideDir(base, target) {
		return "", fmt.Errorf("upload path escapes upload directory")
	}
	return target, nil
}
