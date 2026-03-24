package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type RouteEntry struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
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
	StaticRoot string `json:"static_root"`
	LogPath    string `json:"log_path"`
	DBPath     string `json:"db_path"`
}

type ServerState struct {
	PID        int    `json:"pid"`
	Port       string `json:"port"`
	LogPath    string `json:"log_path"`
	StaticRoot string `json:"static_root"`
	StartedAt  string `json:"started_at"`
}

var (
	routeConfigPath = filepath.Join(os.TempDir(), "squ1dweb_routes.json")
	settingsPath    = filepath.Join(os.TempDir(), "squ1dweb_settings.json")
	serverStatePath = filepath.Join(os.TempDir(), "squ1dweb_server_state.json")

	routeMutex    sync.RWMutex
	settingsMutex sync.Mutex
	logMutex      sync.Mutex

	squ1dccPath = os.Getenv("SQU1DCC_BIN")
	projectRoot = ""
	staticRoot  = ""
	logFilePath = ""
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
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__server__" {
		if len(os.Args) > 2 {
			startWebServer(os.Args[2])
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

		SQXMethod{Name: "response", Return: SQXReturnString, Handle: responseHandler},
		SQXMethod{Name: "response_json", Return: SQXReturnString, Handle: responseJSONHandler},
		SQXMethod{Name: "response_html", Return: SQXReturnString, Handle: responseHTMLHandler},
		SQXMethod{Name: "response_text", Return: SQXReturnString, Handle: responseTextHandler},
		SQXMethod{Name: "render_file", Return: SQXReturnString, Handle: renderFileHandler},

		SQXMethod{Name: "parse_form", Return: SQXReturnJSON, Handle: parseFormHandler},
		SQXMethod{Name: "form_get", Return: SQXReturnString, Handle: formGetHandler},
		SQXMethod{Name: "parse_headers", Return: SQXReturnJSON, Handle: parseHeadersHandler},
		SQXMethod{Name: "header_get", Return: SQXReturnString, Handle: headerGetHandler},

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
	cmd.Env = append(os.Environ(),
		"SQU1DWEB_STATIC_ROOT="+settings.StaticRoot,
		"SQU1DWEB_LOG_FILE="+settings.LogPath,
	)
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
	}
	if err := saveServerState(state); err != nil {
		return nil, err
	}

	return fmt.Sprintf("Server started on port %s (PID: %d, logs: %s)", port, state.PID, settings.LogPath), nil
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
	}, nil
}

func setRouteHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 3); err != nil {
		return nil, err
	}

	method := strings.ToUpper(strings.TrimSpace(args[0]))
	path := strings.TrimSpace(args[1])
	handler := strings.TrimSpace(args[2])
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
			updated = true
			break
		}
	}
	if !updated {
		cfg.Routes = append(cfg.Routes, RouteEntry{Method: method, Path: path, Handler: handler})
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
	if fromEnv := strings.TrimSpace(os.Getenv("SQU1DWEB_STATIC_ROOT")); fromEnv != "" {
		settings.StaticRoot = fromEnv
	}
	if fromEnv := strings.TrimSpace(os.Getenv("SQU1DWEB_LOG_FILE")); fromEnv != "" {
		settings.LogPath = fromEnv
	}
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
	resp, err := executeRouteHandler(matched.Handler, r, body)
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

func executeRouteHandler(handlerPath string, r *http.Request, requestBody []byte) (*RouteResponse, error) {
	if handlerPath == "" {
		return nil, fmt.Errorf("empty handler path")
	}

	absHandlerPath := handlerPath
	if !filepath.IsAbs(absHandlerPath) {
		absHandlerPath = resolvePath(handlerPath)
	}

	headersJSON, _ := json.Marshal(flattenHeader(r.Header))
	cmd := exec.Command(squ1dccPath, absHandlerPath)
	cmd.Dir = findProjectRoot(absHandlerPath)
	cmd.Env = append(os.Environ(),
		"HTTP_METHOD="+r.Method,
		"HTTP_PATH="+r.URL.Path,
		"HTTP_QUERY="+r.URL.RawQuery,
		"HTTP_BODY="+string(requestBody),
		"HTTP_HEADERS_JSON="+string(headersJSON),
	)
	cmd.Stdin = bytes.NewReader(requestBody)

	out, err := cmd.CombinedOutput()
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
		StaticRoot: resolvePath("static"),
		LogPath:    resolvePath("squ1dweb.log"),
		DBPath:     resolvePath("squ1dweb.db"),
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
	settings.StaticRoot = resolvePath(settings.StaticRoot)
	settings.LogPath = resolvePath(settings.LogPath)
	settings.DBPath = resolvePath(settings.DBPath)
	return settings
}

func saveSettings(settings EngineSettings) error {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	settings.StaticRoot = resolvePath(settings.StaticRoot)
	settings.LogPath = resolvePath(settings.LogPath)
	settings.DBPath = resolvePath(settings.DBPath)

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
	if fromEnv := strings.TrimSpace(os.Getenv("SQU1DWEB_ROOT")); fromEnv != "" {
		return filepath.Clean(fromEnv)
	}

	exe, err := os.Executable()
	start := ""
	if err == nil {
		start = filepath.Dir(exe)
	}
	if start == "" {
		cwd, err := os.Getwd()
		if err == nil {
			start = cwd
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
