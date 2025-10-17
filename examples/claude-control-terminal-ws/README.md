# Claude Control Terminal WebSocket Example

A WebSocket server that integrates with the Claude Agent SDK to provide remote access to Claude AI through WebSocket connections. This example demonstrates how to build a production-ready service using the SDK with concurrent session management, graceful shutdown, and proper error handling.

## What This Example Demonstrates

1. **WebSocket Server Integration**: Building an HTTP/WebSocket server that uses the Claude Agent SDK
2. **Concurrent Session Management**: Handling multiple simultaneous Claude queries with session limits
3. **Lifecycle Management**: Start/stop/status commands with PID file tracking
4. **Streaming Responses**: Converting SDK message channels to WebSocket JSON streams
5. **Configuration Management**: Environment variables and command-line flags
6. **Error Handling**: Proper error propagation and client error messages
7. **Graceful Shutdown**: Context-based cancellation and resource cleanup

## Prerequisites

- Go 1.20 or higher
- Claude Code CLI installed: `npm install -g @anthropic-ai/claude-code`
- Valid Claude API key (set in `CLAUDE_API_KEY` environment variable)

## Installation

```bash
# Clone the repository
cd claude-agent-sdk-go/examples/claude-control-terminal-ws

# Build the binary
go build -o claude-control-terminal-ws

# Or install to GOPATH/bin
go install
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_API_KEY` | *required* | Your Claude API key |
| `AGENT_WS_HOST` | `127.0.0.1` | Server bind address |
| `AGENT_WS_PORT` | `8080` | Server port |
| `AGENT_WS_MODEL` | `claude-3-5-sonnet-latest` | Claude model to use |
| `AGENT_WS_LOG_LEVEL` | `INFO` | Log level |
| `AGENT_WS_MAX_CONCURRENT_SESSIONS` | `10` | Maximum concurrent WebSocket sessions |

### Command-Line Flags

#### Start Command
```bash
--host string    Server host (overrides AGENT_WS_HOST)
--port int       Server port (overrides AGENT_WS_PORT)
--model string   Claude model (overrides AGENT_WS_MODEL)
--quiet          Suppress output
```

#### Logs Command
```bash
--follow         Follow log output (like tail -f)
--lines int      Number of lines to show (default: 50)
```

## Usage

### Start Server

```bash
# Start with defaults
./claude-control-terminal-ws start

# Start on custom port with specific model
./claude-control-terminal-ws start --port 9090 --model claude-3-opus-latest

# Start in background (daemonize)
nohup ./claude-control-terminal-ws start &
```

Output:
```
Starting WebSocket server on ws://127.0.0.1:8080
Server started successfully (PID: 12345)
  Endpoint: ws://127.0.0.1:8080/ws
  Health: ws://127.0.0.1:8080/health
  Model: claude-3-5-sonnet-latest
  Logs: /Users/you/.claude/agents-sdk-ws/server.log
```

### Check Status

```bash
./claude-control-terminal-ws status
```

Output:
```
Server is running (PID: 12345)
Endpoint: ws://127.0.0.1:8080/ws
Health: ws://127.0.0.1:8080/health
Model: claude-3-5-sonnet-latest
Logs: /Users/you/.claude/agents-sdk-ws/server.log
```

### View Logs

```bash
# Show last 50 lines
./claude-control-terminal-ws logs

# Show last 100 lines
./claude-control-terminal-ws logs --lines 100

# Follow logs (like tail -f)
./claude-control-terminal-ws logs --follow
```

### Stop Server

```bash
./claude-control-terminal-ws stop
```

## WebSocket Protocol

### Connection

Connect to: `ws://localhost:8080/ws`

### Request Format

Send JSON messages with a `prompt` field:

```json
{
  "prompt": "What is 2 + 2?"
}
```

### Response Format

Responses are streamed as JSON messages:

```json
{
  "type": "assistant",
  "content": {
    "text": ["The answer is 4."]
  }
}
```

Response types:
- `assistant`: Claude's response with text content
- `user`: Echo of user's message
- `result`: Final result with cost and token usage
- `system`: System messages
- `error`: Error occurred

#### Example Response Stream

```json
{"type": "user", "content": "What is 2 + 2?"}
{"type": "assistant", "content": {"text": ["The answer is 4."]}}
{"type": "result", "content": {"success": true, "cost_usd": 0.0012, "input_tokens": 10, "output_tokens": 8}}
```

### Error Responses

```json
{
  "type": "error",
  "error": "prompt cannot be empty"
}
```

## Client Examples

### JavaScript/Node.js

```javascript
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8080/ws');

ws.on('open', () => {
  ws.send(JSON.stringify({ prompt: 'What is 2 + 2?' }));
});

ws.on('message', (data) => {
  const msg = JSON.parse(data);
  console.log('Received:', msg);

  if (msg.type === 'result') {
    ws.close();
  }
});
```

### Python

```python
import asyncio
import json
import websockets

async def query_claude():
    uri = "ws://localhost:8080/ws"
    async with websockets.connect(uri) as websocket:
        # Send query
        await websocket.send(json.dumps({
            "prompt": "What is 2 + 2?"
        }))

        # Receive responses
        while True:
            response = await websocket.recv()
            msg = json.loads(response)
            print("Received:", msg)

            if msg["type"] == "result":
                break

asyncio.run(query_claude())
```

### curl with websocat

```bash
# Install websocat: https://github.com/vi/websocat
websocat ws://localhost:8080/ws

# Type and send:
{"prompt": "Hello Claude!"}
```

### Go

```go
package main

import (
	"encoding/json"
	"log"

	"golang.org/x/net/websocket"
)

func main() {
	ws, err := websocket.Dial("ws://localhost:8080/ws", "", "http://localhost/")
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Send query
	query := map[string]string{"prompt": "What is 2 + 2?"}
	if err := websocket.JSON.Send(ws, query); err != nil {
		log.Fatal(err)
	}

	// Receive responses
	for {
		var msg map[string]interface{}
		if err := websocket.JSON.Receive(ws, &msg); err != nil {
			log.Fatal(err)
		}

		log.Printf("Received: %+v", msg)

		if msg["type"] == "result" {
			break
		}
	}
}
```

## Health Check

The server provides a health check endpoint:

```bash
websocat ws://localhost:8080/health
```

Response:
```json
{
  "active_sessions": 2,
  "max_sessions": 10
}
```

## Architecture

### Components

1. **Config** (`config.go`): Configuration management with environment variable support
2. **Launcher** (`launcher.go`): Server lifecycle management (start/stop/status/logs)
3. **AgentHandler** (`agent_handler.go`): WebSocket handler with SDK integration
4. **Main** (`main.go`): CLI entry point with command routing

### Key Patterns Demonstrated

#### Goroutine Management
```go
// Context-based cancellation for all goroutines
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    select {
    case <-ctx.Done():
        return
    case msg := <-messages:
        // Process message
    }
}()
```

#### Concurrent Session Limiting
```go
type AgentHandler struct {
    mu     sync.Mutex
    active int
}

func (h *AgentHandler) HandleWebSocket(ws *websocket.Conn) {
    h.mu.Lock()
    if h.active >= h.config.MaxConcurrentSessions {
        h.mu.Unlock()
        h.sendError(ws, "max concurrent sessions reached")
        return
    }
    h.active++
    h.mu.Unlock()

    defer func() {
        h.mu.Lock()
        h.active--
        h.mu.Unlock()
    }()

    // Handle connection...
}
```

#### SDK Integration
```go
// Create options
opts := types.NewClaudeAgentOptions().
    WithModel(h.config.Model).
    WithPermissionMode(types.PermissionModeBypassPermissions)

// Execute query
messages, err := claude.Query(ctx, prompt, opts)
if err != nil {
    return err
}

// Stream responses
for msg := range messages {
    h.sendMessage(ws, msg)
}
```

#### Graceful Shutdown
```go
// HTTP server graceful shutdown
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := server.Shutdown(shutdownCtx); err != nil {
    return err
}
```

## File Structure

```
claude-control-terminal-ws/
├── README.md           # This file
├── main.go             # CLI entry point
├── config.go           # Configuration management
├── launcher.go         # Server lifecycle management
└── agent_handler.go    # WebSocket handler with SDK integration
```

## Troubleshooting

### Server won't start

1. **Check if port is in use:**
   ```bash
   lsof -i :8080
   ```

2. **Check if API key is set:**
   ```bash
   echo $CLAUDE_API_KEY
   ```

3. **Check logs:**
   ```bash
   ./claude-control-terminal-ws logs
   ```

### Connection refused

- Verify server is running: `./claude-control-terminal-ws status`
- Check firewall settings
- Ensure correct host/port

### High memory usage

- Reduce `AGENT_WS_MAX_CONCURRENT_SESSIONS`
- Check for WebSocket connection leaks
- Monitor with: `ps aux | grep claude-control-terminal-ws`

### Stale PID file

If server status shows "running" but it's not:
```bash
rm ~/.claude/agents-sdk-ws/.pid
```

## Production Considerations

1. **TLS/SSL**: Add HTTPS support for production deployments
2. **Authentication**: Implement token-based auth or API keys
3. **Rate Limiting**: Add per-client rate limiting
4. **Logging**: Integrate with log aggregation (e.g., syslog, CloudWatch)
5. **Monitoring**: Add Prometheus metrics or health endpoints
6. **Deployment**: Use systemd, Docker, or Kubernetes for production
7. **Resource Limits**: Set ulimits and memory limits

## systemd Service Example

Create `/etc/systemd/system/claude-ws.service`:

```ini
[Unit]
Description=Claude WebSocket Server
After=network.target

[Service]
Type=simple
User=youruser
Environment="CLAUDE_API_KEY=your-api-key"
ExecStart=/usr/local/bin/claude-control-terminal-ws start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable claude-ws
sudo systemctl start claude-ws
sudo systemctl status claude-ws
```

## License

This example is part of the Claude Agent SDK for Go and follows the same MIT license.

## Related Examples

- [simple_query](../simple_query/): Basic one-shot queries
- [interactive_client](../interactive_client/): Interactive CLI client
- [with_hooks](../with_hooks/): Using lifecycle hooks
- [with_permissions](../with_permissions/): Permission handling
