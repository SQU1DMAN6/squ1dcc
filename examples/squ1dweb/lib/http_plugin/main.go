package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

var (
	routeConfigPath = filepath.Join(os.TempDir(), "squ1dweb_routes.json")
	routeMutex      sync.RWMutex
	logMutex        sync.Mutex
	squ1dccPath     = os.Getenv("SQU1DCC_BIN")
	staticRoot      = "/home/qchef/Documents/squ1dcc/examples/squ1dweb/static"
	dbMutex         sync.Mutex
	dbConn          *sql.DB
	dbPath          string
)

func init() {
	if squ1dccPath == "" {
		if path, err := exec.LookPath("squ1dcc"); err == nil {
			squ1dccPath = path
		} else {
			squ1dccPath = "squ1dcc"
		}
	}
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
		SQXMethod{Name: "set_route", Return: SQXReturnString, Handle: setRouteHandler},
		SQXMethod{Name: "list_routes", Return: SQXReturnJSON, Handle: listRoutesHandler},
		SQXMethod{Name: "response", Return: SQXReturnString, Handle: responseHandler},
		SQXMethod{Name: "remove_route", Return: SQXReturnString, Handle: removeRouteHandler},
		SQXMethod{Name: "clear_routes", Return: SQXReturnString, Handle: clearRoutesHandler},
		SQXMethod{Name: "db_open", Return: SQXReturnString, Handle: dbOpenHandler},
		SQXMethod{Name: "db_exec", Return: SQXReturnJSON, Handle: dbExecHandler},
		SQXMethod{Name: "db_query", Return: SQXReturnJSON, Handle: dbQueryHandler},
		SQXMethod{Name: "db_close", Return: SQXReturnString, Handle: dbCloseHandler},
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
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	resp, err := http.Get(args[0])
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"status": resp.StatusCode, "length": len(body), "body": string(body), "url": args[0]}, nil
}

func postHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 2); err != nil {
		return nil, err
	}
	url := args[0]
	payload := args[1]
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"status": resp.StatusCode, "length": len(body), "body": string(body), "url": url}, nil
}

func statusHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	resp, err := http.Head(args[0])
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
	port := args[0]
	cmd := exec.Command(os.Args[0], "__server__", port)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("Failed to start server on port %s: %v", port, err), nil
	}
	return fmt.Sprintf("Server started on port %s (PID: %d)", port, cmd.Process.Pid), nil
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
	saveRouteConfig(cfg)
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
	filtered := []RouteEntry{}
	for _, r := range cfg.Routes {
		if strings.EqualFold(r.Method, method) && r.Path == path {
			continue
		}
		filtered = append(filtered, r)
	}
	cfg.Routes = filtered
	saveRouteConfig(cfg)
	return fmt.Sprintf("Removed route %s %s", method, path), nil
}

func clearRoutesHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	routeMutex.Lock()
	defer routeMutex.Unlock()
	saveRouteConfig(RouteConfig{})
	return "Cleared all routes", nil
}

func dbOpenHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	path := strings.TrimSpace(args[0])
	if path == "" {
		return nil, fmt.Errorf("db path cannot be empty")
	}
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if dbConn != nil {
		_ = dbConn.Close()
	}
	var err error
	dbConn, err = sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	dbPath = path
	return fmt.Sprintf("DB opened %s", path), nil
}

func dbExecHandler(args []string) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("expected at least 1 argument")
	}
	query := args[0]
	params := []interface{}{}
	if len(args) > 1 && args[1] != "" {
		if err := json.Unmarshal([]byte(args[1]), &params); err != nil {
			return nil, fmt.Errorf("invalid params JSON: %v", err)
		}
	}
	dbPathArg := "/tmp/squ1dweb.db"
	if len(args) > 2 && args[2] != "" {
		dbPathArg = args[2]
	}
	db, err := sql.Open("sqlite3", dbPathArg)
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
	return map[string]interface{}{"rows_affected": rows, "last_insert_id": lastID}, nil
}

func dbQueryHandler(args []string) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("expected at least 1 argument")
	}
	query := args[0]
	params := []interface{}{}
	if len(args) > 1 && args[1] != "" {
		if err := json.Unmarshal([]byte(args[1]), &params); err != nil {
			return nil, fmt.Errorf("invalid params JSON: %v", err)
		}
	}
	dbPathArg := "/tmp/squ1dweb.db"
	if len(args) > 2 && args[2] != "" {
		dbPathArg = args[2]
	}
	db, err := sql.Open("sqlite3", dbPathArg)
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
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}
		rowMap := map[string]interface{}{}
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			rowMap[colName] = *val
		}
		results = append(results, rowMap)
	}
	return results, nil
}

func responseHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 3); err != nil {
		return nil, err
	}
	statusRaw := args[0]
	contentType := args[1]
	body := args[2]
	if statusRaw == "" {
		statusRaw = "200"
	}
	return fmt.Sprintf("{\"status\":%s,\"content_type\":\"%s\",\"body\":\"%s\"}", statusRaw, contentType, strings.ReplaceAll(strings.ReplaceAll(body, "\\", "\\\\"), "\"", "\\\"")), nil
}

func dbCloseHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if dbConn != nil {
		_ = dbConn.Close()
		dbConn = nil
	}
	return "DB closed", nil
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

func saveRouteConfig(cfg RouteConfig) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logWebEvent("Failed to marshal route config: " + err.Error())
		return
	}
	_ = os.WriteFile(routeConfigPath, data, 0644)
}

func startWebServer(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", genericHandler)

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logWebEvent("Listen error: " + err.Error())
		return
	}
	defer ln.Close()

	logWebEvent("Server listening on :" + port)
	srv := &http.Server{Handler: loggingMiddleware(authMiddleware(mux))}
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
		w.Write([]byte(b))
	case []byte:
		w.Write(b)
	default:
		if out, err := json.Marshal(b); err == nil {
			w.Write(out)
		} else {
			w.Write([]byte(fmt.Sprintf("%v", b)))
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
	trimmed := strings.TrimPrefix(r.URL.Path, "/static/")
	if trimmed == "" {
		trimmed = "index.html"
	}
	fname := filepath.Join(staticRoot, filepath.FromSlash(trimmed))
	if info, err := os.Stat(fname); err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fname)
}

func executeRouteHandler(handlerPath string, r *http.Request, requestBody []byte) (*RouteResponse, error) {
	if handlerPath == "" {
		return nil, fmt.Errorf("empty handler path")
	}

	absHandlerPath := handlerPath
	if !filepath.IsAbs(absHandlerPath) {
		p, err := filepath.Abs(handlerPath)
		if err != nil {
			absHandlerPath = handlerPath
		} else {
			absHandlerPath = p
		}
	}

	cmd := exec.Command(squ1dccPath, absHandlerPath)
	cmd.Dir = filepath.Clean(filepath.Join(filepath.Dir(absHandlerPath), ".."))
	cmd.Env = append(os.Environ(),
		"HTTP_METHOD="+r.Method,
		"HTTP_PATH="+r.URL.Path,
		"HTTP_QUERY="+r.URL.RawQuery,
		"HTTP_BODY="+string(requestBody),
	)
	cmd.Stdin = bytes.NewReader(requestBody)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("route handler %s error: %v output=%s", absHandlerPath, err, strings.TrimSpace(string(out)))
	}

	var routeResp RouteResponse
	if err := json.Unmarshal(out, &routeResp); err == nil {
		if routeResp.Status == 0 {
			routeResp.Status = 200
		}
		return &routeResp, nil
	}

	// Fallback for map objects (SQD object style) or plain text body
	var generic map[string]interface{}
	if err := json.Unmarshal(out, &generic); err == nil {
		var body interface{} = ""
		if b, ok := generic["body"]; ok {
			body = b
		}
		status := 200
		if s, ok := generic["status"].(float64); ok {
			status = int(s)
		}
		contentType := "text/plain"
		if ct, ok := generic["content_type"].(string); ok {
			contentType = ct
		}
		headers := map[string]string{}
		if h, ok := generic["headers"].(map[string]interface{}); ok {
			for k, v := range h {
				if vs, ok := v.(string); ok {
					headers[k] = vs
				}
			}
		}
		return &RouteResponse{Status: status, ContentType: contentType, Body: body, Headers: headers}, nil
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return &RouteResponse{Status: 204, ContentType: "text/plain", Body: ""}, nil
	}

	return &RouteResponse{Status: 200, ContentType: "text/plain", Body: string(out)}, nil
}

// loggingResponseWriter and middleware

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

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/dashboard") {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized: add Authorization header"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}
