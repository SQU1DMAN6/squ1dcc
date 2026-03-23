package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Global user store and sessions
var (
	users    = make(map[string]map[string]string) // username -> {password, email}
	sessions = make(map[string]string)            // sessionID -> username
	mu       sync.RWMutex
	ready    = make(chan bool, 1) // Signal when server is listening
)

func main() {
	// If called with "__server__" argument, run as persistent HTTP server
	if len(os.Args) > 1 && os.Args[1] == "__server__" {
		if len(os.Args) > 2 {
			port := os.Args[2]
			startWebServer(port)
		}
		return
	}

	// Otherwise, run as SQX plugin module
	module := NewSQXModule("http")

	module.RegisterMany(
		SQXMethod{
			Name:   "ping",
			Return: SQXReturnString,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 0); err != nil {
					return nil, err
				}
				return "http pong", nil
			},
		},
		SQXMethod{
			Name:   "get",
			Return: SQXReturnJSON,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}
				url := args[0]
				resp, err := http.Get(url)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{
					"status": resp.StatusCode,
					"length": len(body),
					"body":   string(body),
					"url":    url,
				}, nil
			},
		},
		SQXMethod{
			Name:   "status",
			Return: SQXReturnInt,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}
				url := args[0]
				resp, err := http.Head(url)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				return resp.StatusCode, nil
			},
		},
		SQXMethod{
			Name:   "server_start",
			Return: SQXReturnString,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}
				port := args[0]

				// Start server as separate persistent process
				cmd := exec.Command(os.Args[0], "__server__", port)
				cmd.Stdout = os.Stderr
				cmd.Stderr = os.Stderr

				err := cmd.Start()
				if err != nil {
					return fmt.Sprintf("Failed to start server on port %s: %v", port, err), nil
				}

				// Give server time to start
				time.Sleep(500 * time.Millisecond)

				// Verify port is listening
				timeout := time.Now().Add(3 * time.Second)
				for {
					if timeout.Before(time.Now()) {
						break
					}

					listener, err := net.Listen("tcp", ":"+port)
					if err == nil {
						listener.Close()
						// Port is open, server started successfully
						return fmt.Sprintf("Server started on port %s (PID: %d)", port, cmd.Process.Pid), nil
					}

					time.Sleep(100 * time.Millisecond)
				}

				return fmt.Sprintf("Server starting on port %s (PID: %d) - may take a moment", port, cmd.Process.Pid), nil
			},
		},
		SQXMethod{
			Name:   "register",
			Return: SQXReturnJSON,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 2); err != nil {
					return nil, err
				}

				// Decode typed arguments
				usernameVal, err := SQXDecodeArg(args[0])
				if err != nil {
					return nil, err
				}
				username := strings.TrimSpace(fmt.Sprint(usernameVal))

				passwordVal, err := SQXDecodeArg(args[1])
				if err != nil {
					return nil, err
				}
				password := fmt.Sprint(passwordVal)

				mu.Lock()
				defer mu.Unlock()

				if _, exists := users[username]; exists {
					return map[string]interface{}{"ok": false, "error": "User already exists"}, nil
				}

				users[username] = map[string]string{
					"password": hashPassword(password),
				}

				return map[string]interface{}{"ok": true, "username": username}, nil
			},
		},
		SQXMethod{
			Name:   "login",
			Return: SQXReturnJSON,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 2); err != nil {
					return nil, err
				}

				// Decode typed arguments
				usernameVal, err := SQXDecodeArg(args[0])
				if err != nil {
					return nil, err
				}
				username := strings.TrimSpace(fmt.Sprint(usernameVal))

				passwordVal, err := SQXDecodeArg(args[1])
				if err != nil {
					return nil, err
				}
				password := fmt.Sprint(passwordVal)

				mu.RLock()
				user, exists := users[username]
				mu.RUnlock()

				if !exists || user["password"] != hashPassword(password) {
					return map[string]interface{}{"ok": false, "error": "Invalid credentials"}, nil
				}

				sessionID := generateSessionID()
				mu.Lock()
				sessions[sessionID] = username
				mu.Unlock()

				return map[string]interface{}{
					"ok":        true,
					"sessionID": sessionID,
					"username":  username,
				}, nil
			},
		},
		SQXMethod{
			Name:   "verify_session",
			Return: SQXReturnJSON,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}

				// Decode typed argument
				sessionVal, err := SQXDecodeArg(args[0])
				if err != nil {
					return nil, err
				}
				sessionID := fmt.Sprint(sessionVal)

				mu.RLock()
				username, exists := sessions[sessionID]
				mu.RUnlock()

				if !exists {
					return map[string]interface{}{"ok": false}, nil
				}

				return map[string]interface{}{
					"ok":       true,
					"username": username,
				}, nil
			},
		},
	)

	os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateSessionID() string {
	hash := sha256.Sum256([]byte(time.Now().String()))
	return hex.EncodeToString(hash[:])[:16]
}

func startWebServer(port string) {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/dashboard", handleDashboard)
	http.HandleFunc("/api/verify", handleVerify)
	http.HandleFunc("/static/", handleStatic)

	// Create listener first to ensure port is available
	addr := ":" + port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to listen on %s: %v\n", addr, err)
		return
	}
	defer listener.Close()

	// Signal that server is ready NOW (port is bound)
	go func() {
		select {
		case ready <- true:
		default:
		}
	}()

	fmt.Fprintf(os.Stderr, "🚀 Server listening on port %s\n", port)

	// Create and start server on the listener
	server := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "❌ Server error: %v\n", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(getLoginHTML()))
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := users[req.Username]; exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "User already exists",
		})
		return
	}

	users[req.Username] = map[string]string{
		"password": hashPassword(req.Password),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"username": req.Username,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.RLock()
	user, exists := users[req.Username]
	mu.RUnlock()

	if !exists || user["password"] != hashPassword(req.Password) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "Invalid credentials",
		})
		return
	}

	sessionID := generateSessionID()
	mu.Lock()
	sessions[sessionID] = req.Username
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"sessionID": sessionID,
		"username":  req.Username,
	})
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")

	mu.RLock()
	username, exists := sessions[sessionID]
	mu.RUnlock()

	if !exists {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(getDashboardHTML(username)))
}

func handleVerify(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")

	mu.RLock()
	username, exists := sessions[sessionID]
	mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if exists {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"username": username,
		})
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false,
		})
	}
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")

	switch path {
	case "style.css":
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(getCSS()))
	case "script.js":
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(getJS()))
	default:
		http.NotFound(w, r)
	}
}

func getLoginHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>SQU1D++ Web Auth</title>
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<div class="container">
		<div class="auth-box">
			<h1>SQU1D++ Web</h1>
			<div id="tab-buttons">
				<button onclick="showTab('login')" class="tab-btn active" id="login-btn">Login</button>
				<button onclick="showTab('register')" class="tab-btn" id="register-btn">Register</button>
			</div>

			<!-- Login Tab -->
			<div id="login" class="tab-content active">
				<form id="login-form" onsubmit="handleLogin(event)">
					<input type="text" placeholder="Username" id="login-username" required>
					<input type="password" placeholder="Password" id="login-password" required>
					<button type="submit">Login</button>
					<p id="login-message"></p>
				</form>
			</div>

			<!-- Register Tab -->
			<div id="register" class="tab-content">
				<form id="register-form" onsubmit="handleRegister(event)">
					<input type="text" placeholder="Username" id="register-username" required>
					<input type="password" placeholder="Password" id="register-password" required>
					<button type="submit">Register</button>
					<p id="register-message"></p>
				</form>
			</div>
		</div>
	</div>
	<script src="/static/script.js"></script>
</body>
</html>
`
}

func getDashboardHTML(username string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Dashboard - SQU1D++</title>
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<div class="container">
		<div class="dashboard-box">
			<h1>Welcome, %s!</h1>
			<p>You are logged into SQU1D++ Web Server</p>
			<button onclick="logout()">Logout</button>
		</div>
	</div>
	<script src="/static/script.js"></script>
	<script>
		// Verify session is still valid
		const params = new URLSearchParams(window.location.search);
		const sessionID = params.get('session');
		if (!sessionID) {
			window.location.href = '/';
		}
	</script>
</body>
</html>
`, username)
}

func getCSS() string {
	return `* {
	margin: 0;
	padding: 0;
	box-sizing: border-box;
}

body {
	font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
	background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
	min-height: 100vh;
	display: flex;
	justify-content: center;
	align-items: center;
}

.container {
	width: 100%;
	max-width: 400px;
	padding: 20px;
}

.auth-box, .dashboard-box {
	background: white;
	border-radius: 10px;
	box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
	padding: 40px;
	text-align: center;
}

h1 {
	color: #333;
	margin-bottom: 30px;
	font-size: 28px;
}

#tab-buttons {
	display: flex;
	gap: 10px;
	margin-bottom: 20px;
}

.tab-btn {
	flex: 1;
	padding: 10px;
	border: none;
	background: #f0f0f0;
	cursor: pointer;
	border-radius: 5px;
	font-weight: 600;
	transition: all 0.3s;
}

.tab-btn.active {
	background: #667eea;
	color: white;
}

.tab-content {
	display: none;
}

.tab-content.active {
	display: block;
}

form {
	display: flex;
	flex-direction: column;
	gap: 15px;
}

input {
	padding: 12px;
	border: 1px solid #ddd;
	border-radius: 5px;
	font-size: 14px;
	transition: border-color 0.3s;
}

input:focus {
	outline: none;
	border-color: #667eea;
}

button {
	padding: 12px;
	border: none;
	background: #667eea;
	color: white;
	border-radius: 5px;
	font-weight: 600;
	cursor: pointer;
	transition: background 0.3s;
	font-size: 14px;
}

button:hover {
	background: #764ba2;
}

p {
	margin: 10px 0;
	font-size: 14px;
	min-height: 20px;
}

.success {
	color: #4caf50;
}

.error {
	color: #f44336;
}

.dashboard-box button {
	margin-top: 20px;
}
`
}

func getJS() string {
	return `function showTab(tabName) {
	const tabs = document.querySelectorAll('.tab-content');
	const btns = document.querySelectorAll('.tab-btn');
	
	tabs.forEach(tab => tab.classList.remove('active'));
	btns.forEach(btn => btn.classList.remove('active'));
	
	document.getElementById(tabName).classList.add('active');
	document.getElementById(tabName + '-btn').classList.add('active');
}

function handleLogin(e) {
	e.preventDefault();
	const username = document.getElementById('login-username').value;
	const password = document.getElementById('login-password').value;
	const messageEl = document.getElementById('login-message');
	
	fetch('/api/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ username, password })
	})
	.then(r => r.json())
	.then(data => {
		if (data.ok) {
			messageEl.textContent = 'Login successful!';
			messageEl.className = 'success';
			setTimeout(() => {
				window.location.href = '/api/dashboard?session=' + data.sessionID;
			}, 1000);
		} else {
			messageEl.textContent = data.error || 'Login failed';
			messageEl.className = 'error';
		}
	})
	.catch(err => {
		messageEl.textContent = 'Error: ' + err.message;
		messageEl.className = 'error';
	});
}

function handleRegister(e) {
	e.preventDefault();
	const username = document.getElementById('register-username').value;
	const password = document.getElementById('register-password').value;
	const messageEl = document.getElementById('register-message');
	
	fetch('/api/register', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ username, password })
	})
	.then(r => r.json())
	.then(data => {
		if (data.ok) {
			messageEl.textContent = 'Registration successful! Please login.';
			messageEl.className = 'success';
			setTimeout(() => {
				document.getElementById('register-username').value = '';
				document.getElementById('register-password').value = '';
				showTab('login');
			}, 1000);
		} else {
			messageEl.textContent = data.error || 'Registration failed';
			messageEl.className = 'error';
		}
	})
	.catch(err => {
		messageEl.textContent = 'Error: ' + err.message;
		messageEl.className = 'error';
	});
}

function logout() {
	window.location.href = '/';
}
`
}
