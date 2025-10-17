package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

// Launcher manages the WebSocket server lifecycle
type Launcher struct {
	Config  *Config
	Quiet   bool
	server  *http.Server
	ctx     context.Context
	cancel  context.CancelFunc
	handler *AgentHandler
}

// NewLauncher creates a new launcher instance
func NewLauncher(config *Config, quiet bool) *Launcher {
	return &Launcher{
		Config: config,
		Quiet:  quiet,
	}
}

// log prints a message unless in quiet mode
func (l *Launcher) log(format string, args ...interface{}) {
	if !l.Quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// IsRunning checks if the server is currently running
func (l *Launcher) IsRunning() (bool, int, error) {
	// Read PID file
	pid, err := l.readPIDFile()
	if err != nil {
		return false, 0, nil // No PID file means not running
	}

	// Check if process is running
	if !isProcessRunning(pid) {
		// Stale PID file, remove it
		_ = os.Remove(l.Config.PIDFile)
		return false, 0, nil
	}

	return true, pid, nil
}

// Start starts the WebSocket server
func (l *Launcher) Start() error {
	// Check if already running
	running, pid, _ := l.IsRunning()
	if running {
		l.log("Server already running (PID: %d)", pid)
		return nil
	}

	// Ensure server directory exists
	if err := os.MkdirAll(l.Config.ServerDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	// Validate API key
	if l.Config.APIKey == "" {
		return fmt.Errorf("CLAUDE_API_KEY environment variable is required")
	}

	// Create context for server lifecycle
	l.ctx, l.cancel = context.WithCancel(context.Background())

	// Create agent handler
	l.handler = NewAgentHandler(l.Config)

	// Set up HTTP server with WebSocket endpoints
	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(l.handler.HandleWebSocket))
	mux.Handle("/health", websocket.Handler(l.handler.HealthCheck))

	addr := net.JoinHostPort(l.Config.Host, strconv.Itoa(l.Config.Port))
	l.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	l.log("Starting WebSocket server on ws://%s", addr)

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := l.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait briefly to check for immediate startup errors
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start server: %w", err)
	case <-time.After(500 * time.Millisecond):
		// Server started successfully
	}

	// Verify the server is actually listening
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		_ = l.server.Shutdown(context.Background())
		return fmt.Errorf("server failed to listen on %s: %w", addr, err)
	}
	_ = conn.Close()

	// Write PID file
	if err := l.writePIDFile(os.Getpid()); err != nil {
		_ = l.server.Shutdown(context.Background())
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	l.log("Server started successfully (PID: %d)", os.Getpid())
	l.log("  Endpoint: ws://%s/ws", addr)
	l.log("  Health: ws://%s/health", addr)
	l.log("  Model: %s", l.Config.Model)
	l.log("  Logs: %s", l.Config.LogFile)

	return nil
}

// Stop stops the WebSocket server
func (l *Launcher) Stop() error {
	running, pid, _ := l.IsRunning()
	if !running {
		l.log("Server is not running")
		return nil
	}

	l.log("Stopping server (PID: %d)...", pid)

	// If this process owns the server
	if pid == os.Getpid() && l.server != nil {
		return l.Cleanup()
	}

	// Otherwise, kill the process
	if err := killProcess(pid); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	// Wait for process to stop (up to 10 seconds)
	for i := 0; i < 10; i++ {
		if !isProcessRunning(pid) {
			_ = os.Remove(l.Config.PIDFile)
			l.log("Server stopped")
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	// Force kill if it didn't stop
	l.log("Server did not stop gracefully, force killing...")
	if err := forceKillProcess(pid); err != nil {
		return fmt.Errorf("failed to force kill server: %w", err)
	}

	_ = os.Remove(l.Config.PIDFile)
	l.log("Server stopped (forced)")
	return nil
}

// Cleanup gracefully shuts down the server
func (l *Launcher) Cleanup() error {
	if l.server == nil {
		return nil
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := l.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	// Cancel server context
	if l.cancel != nil {
		l.cancel()
	}

	// Remove PID file
	_ = os.Remove(l.Config.PIDFile)

	l.log("Server stopped gracefully")
	return nil
}

// Status returns the status of the server
func (l *Launcher) Status() (string, error) {
	running, pid, _ := l.IsRunning()
	if !running {
		return "Server is not running", nil
	}

	addr := net.JoinHostPort(l.Config.Host, strconv.Itoa(l.Config.Port))
	return fmt.Sprintf("Server is running (PID: %d)\nEndpoint: ws://%s/ws\nHealth: ws://%s/health\nModel: %s\nLogs: %s",
		pid, addr, addr, l.Config.Model, l.Config.LogFile), nil
}

// Logs tails the server logs
func (l *Launcher) Logs(lines int, follow bool) error {
	logFile := l.Config.LogFile

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", logFile)
	}

	if follow {
		return l.tailLogs(logFile)
	}

	return l.showLastNLines(logFile, lines)
}

// showLastNLines shows the last N lines of the log file
func (l *Launcher) showLastNLines(logFile string, n int) error {
	file, err := os.Open(logFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Print last N lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for i := start; i < len(lines); i++ {
		fmt.Println(lines[i])
	}

	return nil
}

// tailLogs follows the log file and prints new lines
func (l *Launcher) tailLogs(logFile string) error {
	// Show last 10 lines first
	if err := l.showLastNLines(logFile, 10); err != nil {
		return err
	}

	fmt.Println("--- Following logs (Ctrl+C to stop) ---")

	// Open file for reading
	file, err := os.Open(logFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	// Seek to end
	_, _ = file.Seek(0, 2)

	scanner := bufio.NewScanner(file)
	for {
		if scanner.Scan() {
			fmt.Println(scanner.Text())
		} else {
			// No more lines, wait and retry
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// readPIDFile reads the PID from the PID file
func (l *Launcher) readPIDFile() (int, error) {
	data, err := os.ReadFile(l.Config.PIDFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in PID file: %w", err)
	}

	return pid, nil
}

// writePIDFile writes the PID to the PID file
func (l *Launcher) writePIDFile(pid int) error {
	pidDir := filepath.Dir(l.Config.PIDFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(l.Config.PIDFile, []byte(strconv.Itoa(pid)), 0644)
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	if runtime.GOOS == "windows" {
		// On Windows, try to open the process
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		// On Windows, FindProcess always succeeds, so send signal 0
		err = process.Signal(os.Kill)
		return err == nil
	}

	// On Unix, send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(os.Signal(nil))
	return err == nil
}

// killProcess sends SIGTERM to a process
func killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(os.Interrupt)
}

// forceKillProcess sends SIGKILL to a process
func forceKillProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Kill()
}
