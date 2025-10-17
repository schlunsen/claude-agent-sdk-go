package main

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds configuration for the WebSocket agent server
type Config struct {
	Host                  string
	Port                  int
	LogLevel              string
	Model                 string
	APIKey                string
	MaxConcurrentSessions int
	ServerDir             string
	PIDFile               string
	LogFile               string
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	serverDir := filepath.Join(homeDir, ".claude", "agents-sdk-ws")

	return &Config{
		Host:                  getEnvOrDefault("AGENT_WS_HOST", "127.0.0.1"),
		Port:                  getEnvIntOrDefault("AGENT_WS_PORT", 8080),
		LogLevel:              getEnvOrDefault("AGENT_WS_LOG_LEVEL", "INFO"),
		Model:                 getEnvOrDefault("AGENT_WS_MODEL", "claude-3-5-sonnet-latest"),
		APIKey:                getEnvOrDefault("CLAUDE_API_KEY", ""),
		MaxConcurrentSessions: getEnvIntOrDefault("AGENT_WS_MAX_CONCURRENT_SESSIONS", 10),
		ServerDir:             serverDir,
		PIDFile:               filepath.Join(serverDir, ".pid"),
		LogFile:               filepath.Join(serverDir, "server.log"),
	}
}

// GetServerDir returns the agent server directory
func GetServerDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".claude", "agents-sdk-ws")
	}
	return filepath.Join(homeDir, ".claude", "agents-sdk-ws")
}

// getEnvOrDefault returns the environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns the environment variable as int or default
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
