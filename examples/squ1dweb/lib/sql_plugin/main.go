package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type sqlPluginState struct {
	DBPath string `json:"db_path"`
}

var (
	projectRoot = ""
	stateDir    = ""
	stateFile   = ""
)

func init() {
	projectRoot = detectProjectRoot()
	stateDir = filepath.Join(projectRoot, ".sqx")
	stateFile = filepath.Join(stateDir, "sql_plugin_state.json")
}

func main() {
	module := NewSQXModule("sql")

	module.RegisterMany(
		SQXMethod{Name: "ping", Return: SQXReturnString, Handle: pingHandler},
		SQXMethod{Name: "open", Return: SQXReturnString, Handle: openHandler},
		SQXMethod{Name: "close", Return: SQXReturnString, Handle: closeHandler},
		SQXMethod{Name: "db_path", Return: SQXReturnString, Handle: dbPathHandler},
		SQXMethod{Name: "exec", Return: SQXReturnJSON, Handle: execHandler},
		SQXMethod{Name: "query", Return: SQXReturnJSON, Handle: queryHandler},
		SQXMethod{Name: "scalar", Return: SQXReturnJSON, Handle: scalarHandler},
	)

	os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}

func pingHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	return "sql pong", nil
}

func openHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 1); err != nil {
		return nil, err
	}
	path := resolvePath(args[0])
	if path == "" {
		return nil, fmt.Errorf("db path cannot be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, err
	}

	state := loadState()
	state.DBPath = path
	if err := saveState(state); err != nil {
		return nil, err
	}
	return path, nil
}

func closeHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	state := loadState()
	state.DBPath = ""
	if err := saveState(state); err != nil {
		return nil, err
	}
	return "closed", nil
}

func dbPathHandler(args []string) (interface{}, error) {
	if err := SQXRequireArgs(args, 0); err != nil {
		return nil, err
	}
	return loadState().DBPath, nil
}

func execHandler(args []string) (interface{}, error) {
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
	return map[string]interface{}{
		"ok":             true,
		"rows_affected":  rows,
		"last_insert_id": lastID,
		"db_path":        path,
	}, nil
}

func queryHandler(args []string) (interface{}, error) {
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
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			row[col] = normalizeDBValue(values[i])
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func scalarHandler(args []string) (interface{}, error) {
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

	var value interface{}
	if err := db.QueryRow(query, params...).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return normalizeDBValue(value), nil
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
	state := loadState()
	if state.DBPath == "" {
		return "", fmt.Errorf("database is not open; call sql.open(path) first")
	}
	return state.DBPath, nil
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
		projectRoot = detectProjectRoot()
	}
	return filepath.Clean(filepath.Join(projectRoot, trimmed))
}

func loadState() sqlPluginState {
	state := sqlPluginState{}
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	if state.DBPath != "" {
		state.DBPath = resolvePath(state.DBPath)
	}
	return state
}

func saveState(state sqlPluginState) error {
	if state.DBPath != "" {
		state.DBPath = resolvePath(state.DBPath)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0o644)
}

func normalizeDBValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}

func detectProjectRoot() string {
	if fromEnv := strings.TrimSpace(os.Getenv("SQX_PROJECT_ROOT")); fromEnv != "" {
		return filepath.Clean(fromEnv)
	}

	start := ""
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
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
		if fileExists(filepath.Join(current, "main.sqd")) {
			return current
		}
		if dirExists(filepath.Join(current, "routes")) && dirExists(filepath.Join(current, "static")) {
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
