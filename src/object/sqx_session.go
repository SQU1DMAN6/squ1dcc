package object

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"squ1d++/sqx"
)

// sessionCache holds active SQX sessions for executable modules.
// This allows SQU1DCC to reuse persistent sessions instead of
// spawning a new process for every function call.
var (
	sessionCache     = make(map[string]*sqx.Session)
	sessionCachePool = make(map[string]*sqx.SessionPool)
	sessionCacheMu   sync.Mutex
)

// SessionMode controls how SQX modules are executed.
type SessionMode int

const (
	// SessionModeAuto uses sessions when available, falls back to process-per-call.
	SessionModeAuto SessionMode = iota
	// SessionModeAlways forces session mode for all executable SQX modules.
	SessionModeAlways
	// SessionModeLegacy uses process-per-call only (v1 compatibility).
	SessionModeLegacy
)

// SetSessionMode configures the session mode for SQX module loading.
// This can be set from the CLI or configuration.
var CurrentSessionMode = SessionModeAuto

// EnableSessionMode enables session mode for a specific SQX module path.
// Returns the session if successful, or an error if the module doesn't
// support session mode.
func EnableSessionMode(path string) (*sqx.Session, error) {
	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()

	// Check if session already exists
	if sess, ok := sessionCache[path]; ok && !sess.IsClosed() {
		return sess, nil
	}

	// Create a new session
	sess, err := sqx.NewSession(sqx.SessionConfig{
		Path:        path,
		CallTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start SQX session for %q: %w", path, err)
	}

	sessionCache[path] = sess
	return sess, nil
}

// DisableSessionMode closes and removes a session for the given path.
func DisableSessionMode(path string) {
	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()

	if sess, ok := sessionCache[path]; ok {
		sess.Close()
		delete(sessionCache, path)
	}
}

// CloseAllSessions terminates all active SQX sessions.
func CloseAllSessions() {
	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()

	for path, sess := range sessionCache {
		sess.Close()
		delete(sessionCache, path)
	}

	for path, pool := range sessionCachePool {
		pool.CloseAll()
		delete(sessionCachePool, path)
	}
}

// SQXSessionLoader wraps a path to an executable SQX module and
// provides call functionality through either a persistent session
// or legacy process-per-call, depending on configuration.
type SQXSessionLoader struct {
	path      string
	manifest  SQXManifest
	session   *sqx.Session
	pool      *sqx.SessionPool
}

// NewSQXSessionLoader creates a new session loader for an executable SQX module.
func NewSQXSessionLoader(path string, manifest SQXManifest) (*SQXSessionLoader, error) {
	loader := &SQXSessionLoader{
		path:     path,
		manifest: manifest,
	}

	// Attempt to start a session based on the current mode
	if CurrentSessionMode == SessionModeAlways ||
		(CurrentSessionMode == SessionModeAuto && supportsSession(path)) {
		sess, err := EnableSessionMode(path)
		if err == nil {
			loader.session = sess
			return loader, nil
		}
		// If session mode fails, fall back to process-per-call in Auto mode
		if CurrentSessionMode == SessionModeAlways {
			return nil, fmt.Errorf("session mode required but failed for %q: %w", path, err)
		}
	}

	return loader, nil
}

// supportsSession checks if a module supports session mode by
// trying to start a session. This is a best-effort check.
func supportsSession(path string) bool {
	sess, err := sqx.NewSession(sqx.SessionConfig{
		Path:        path,
		CallTimeout: 5 * time.Second,
	})
	if err != nil {
		return false
	}
	sess.Close()
	return true
}

// Call invokes a function through the session or via process-per-call.
func (l *SQXSessionLoader) Call(fnName, returnMode string, args ...Object) (Object, error) {
	if l.session != nil && !l.session.IsClosed() {
		return l.sessionCall(fnName, returnMode, args...)
	}
	return processCall(l.path, fnName, returnMode, args...)
}

// sessionCall invokes a function through the persistent session.
func (l *SQXSessionLoader) sessionCall(fnName, returnMode string, args ...Object) (Object, error) {
	// Convert arguments to strings for the session
	strArgs := make([]string, len(args))
	for i, arg := range args {
		strArgs[i] = sqxArgToString(arg)
	}

	// Call through the session
	result, err := l.session.Call(fnName, strArgs)
	if err != nil {
		return nil, fmt.Errorf("SQX function %q failed: %w", fnName, err)
	}

	// Check for structured error
	if ok, _ := result["ok"].(bool); !ok {
		errMsg := ""
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf("SQX function %q returned error: %s", fnName, errMsg)
	}

	// Extract value and parse according to return mode
	value := result["value"]
	if returnMode == "structured" {
		// For structured return mode, convert the result map to a Hash
		return mapToHash(result), nil
	}

	// For other return modes, convert the value
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session result: %w", err)
	}

	return parseSQXOutput(valueJSON, returnMode)
}

// processCall invokes a function via process-per-call (legacy).
func processCall(modulePath, fnName, returnMode string, args ...Object) (Object, error) {
	return runSQXModuleFunction(modulePath, fnName, returnMode, args...)
}

// mapToHash converts a map[string]interface{} to a Hash object.
func mapToHash(m map[string]interface{}) *Hash {
	pairs := make(map[HashKey]HashPair)
	for k, v := range m {
		key := &String{Value: k}
		pairs[key.HashKey()] = HashPair{Key: key, Value: sqxJSONToObject(v)}
	}
	return NewHash(pairs)
}

// init ensures sessions are cleaned up when the process exits.
func init() {
	// Register cleanup on process exit
	// This is best-effort; the OS will clean up child processes
	// if the parent exits abnormally.
}