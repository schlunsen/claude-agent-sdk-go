package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Parse command
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "start":
		startCommand()
	case "stop":
		stopCommand()
	case "status":
		statusCommand()
	case "logs":
		logsCommand()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func startCommand() {
	// Define flags for start command
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	host := fs.String("host", "", "Server host (default: 127.0.0.1)")
	port := fs.Int("port", 0, "Server port (default: 8080)")
	model := fs.String("model", "", "Claude model (default: claude-3-5-sonnet-latest)")
	quiet := fs.Bool("quiet", false, "Suppress output")

	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	// Load config
	config := DefaultConfig()

	// Override with flags if provided
	if *host != "" {
		config.Host = *host
	}
	if *port != 0 {
		config.Port = *port
	}
	if *model != "" {
		config.Model = *model
	}

	// Create launcher
	launcher := NewLauncher(config, *quiet)

	// Start server
	if err := launcher.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan

	// Cleanup
	if err := launcher.Cleanup(); err != nil {
		log.Printf("Cleanup error: %v", err)
	}
}

func stopCommand() {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	quiet := fs.Bool("quiet", false, "Suppress output")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	config := DefaultConfig()
	launcher := NewLauncher(config, *quiet)

	if err := launcher.Stop(); err != nil {
		log.Fatalf("Failed to stop server: %v", err)
	}
}

func statusCommand() {
	config := DefaultConfig()
	launcher := NewLauncher(config, false)

	status, err := launcher.Status()
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	fmt.Println(status)
}

func logsCommand() {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("follow", false, "Follow log output")
	lines := fs.Int("lines", 50, "Number of lines to show")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	config := DefaultConfig()
	launcher := NewLauncher(config, false)

	if err := launcher.Logs(*lines, *follow); err != nil {
		log.Fatalf("Failed to read logs: %v", err)
	}
}

func printUsage() {
	fmt.Print(`Claude Control Terminal WebSocket Server

A WebSocket server that integrates with the Claude Agent SDK to provide
remote access to Claude AI through WebSocket connections.

USAGE:
    claude-control-terminal-ws <command> [flags]

COMMANDS:
    start       Start the WebSocket server
    stop        Stop the WebSocket server
    status      Show server status
    logs        Show server logs
    help        Show this help message

START FLAGS:
    --host string    Server host (default: 127.0.0.1)
    --port int       Server port (default: 8080)
    --model string   Claude model (default: claude-3-5-sonnet-latest)
    --quiet          Suppress output

LOGS FLAGS:
    --follow         Follow log output (like tail -f)
    --lines int      Number of lines to show (default: 50)

ENVIRONMENT VARIABLES:
    CLAUDE_API_KEY                    Required: Claude API key
    AGENT_WS_HOST                     Server host (default: 127.0.0.1)
    AGENT_WS_PORT                     Server port (default: 8080)
    AGENT_WS_MODEL                    Claude model (default: claude-3-5-sonnet-latest)
    AGENT_WS_LOG_LEVEL                Log level (default: INFO)
    AGENT_WS_MAX_CONCURRENT_SESSIONS  Max concurrent sessions (default: 10)

EXAMPLES:
    # Start server with defaults
    claude-control-terminal-ws start

    # Start on custom port with specific model
    claude-control-terminal-ws start --port 9090 --model claude-3-opus-latest

    # Check server status
    claude-control-terminal-ws status

    # View logs
    claude-control-terminal-ws logs --lines 100

    # Follow logs
    claude-control-terminal-ws logs --follow

    # Stop server
    claude-control-terminal-ws stop

WEBSOCKET PROTOCOL:
    Endpoint: ws://host:port/ws

    Send JSON:
    {
        "prompt": "Your question here"
    }

    Receive JSON:
    {
        "type": "assistant",
        "content": {"text": ["Response text"]},
        "error": ""
    }
`)
}
