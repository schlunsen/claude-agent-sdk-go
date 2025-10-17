#!/bin/bash
# Tool Usage Logger Hook for Claude Code
#
# This hook captures all tool usage (Bash commands and Claude tool invocations)
# and stores them in the CCT analytics database for tracking and analysis.
#
# Hook Type: PostToolUse
# Matcher: * (all tools)
# Input: JSON on stdin with tool_name, session_id, cwd, parameters, result, etc.
# Output: Silent (no stdout/stderr unless error)

set -euo pipefail

# Read JSON from stdin
INPUT=$(cat)

# Parse JSON fields using jq if available, otherwise use grep/sed
if command -v jq &> /dev/null; then
    SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
    CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
    PARAMETERS=$(echo "$INPUT" | jq -c '.tool_input // {}')
    RESULT=$(echo "$INPUT" | jq -c '.tool_response // {}')
    SUCCESS=$(echo "$INPUT" | jq -r 'if .tool_response.interrupted then "false" else "true" end')
    ERROR_MESSAGE=$(echo "$INPUT" | jq -r '.tool_response.error // empty')
    DURATION_MS=$(echo "$INPUT" | jq -r '.tool_response.durationMs // 0')
else
    # Fallback to basic parsing (less robust)
    SESSION_ID=$(echo "$INPUT" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4 || echo "")
    TOOL_NAME=$(echo "$INPUT" | grep -o '"tool_name":"[^"]*"' | cut -d'"' -f4 || echo "")
    CWD=$(echo "$INPUT" | grep -o '"cwd":"[^"]*"' | cut -d'"' -f4 || echo "")
    PARAMETERS="{}"
    RESULT="{}"
    SUCCESS="true"
    ERROR_MESSAGE=""
    DURATION_MS="0"
fi

# Validate required fields
if [[ -z "$SESSION_ID" ]] || [[ -z "$TOOL_NAME" ]] || [[ -z "$CWD" ]]; then
    # Silent failure - don't block Claude Code
    exit 0
fi

# Get git branch from working directory
GIT_BRANCH=""
if [[ -d "$CWD/.git" ]]; then
    GIT_BRANCH=$(cd "$CWD" && git branch --show-current 2>/dev/null || echo "")
fi

# Generate friendly session name from session_id
# Use a list of 25 South Park character names and hash the session_id to pick one
SESSION_NAMES=(
    "Cartman"
    "Stan"
    "Kyle"
    "Kenny"
    "Butters"
    "Randy"
    "Tweek"
    "Craig"
    "Token"
    "Wendy"
    "Sheila"
    "Sharon"
    "Chef"
    "Mr-Garrison"
    "Mr-Mackey"
    "Jimmy"
    "Timmy"
    "Bebe"
    "Clyde"
    "Ike"
    "PC-Principal"
    "Towelie"
    "Mr-Hankey"
    "Big-Gay-Al"
    "Satan"
)

# Generate a numeric hash from session_id to pick a name (modulo 25)
if command -v cksum &> /dev/null; then
    HASH=$(echo -n "$SESSION_ID" | cksum | cut -d' ' -f1)
    INDEX=$((HASH % 25))
else
    # Fallback: use character values
    HASH=0
    for ((i=0; i<${#SESSION_ID} && i<8; i++)); do
        CHAR="${SESSION_ID:$i:1}"
        ASCII=$(printf '%d' "'$CHAR")
        HASH=$((HASH + ASCII))
    done
    INDEX=$((HASH % 25))
fi

SESSION_NAME="${SESSION_NAMES[$INDEX]}"

# Extract model information from environment variables
MODEL_NAME="${ANTHROPIC_MODEL:-}"
MODEL_PROVIDER="${ANTHROPIC_BASE_URL:-https://api.anthropic.com}"

# Analytics server base URL (HTTPS by default)
# Use CCT_ANALYTICS_URL if set, otherwise default to https://localhost:3333
BASE_URL="${CCT_ANALYTICS_URL:-https://localhost:3333}"
SHELL_ENDPOINT="${BASE_URL}/api/commands/shell"
CLAUDE_ENDPOINT="${BASE_URL}/api/commands/claude"

# Read API key from .secret file if it exists
API_KEY_FILE="${CCT_API_KEY_FILE:-$HOME/.claude/analytics/.secret}"
API_KEY=""
if [[ -f "$API_KEY_FILE" ]]; then
    API_KEY=$(cat "$API_KEY_FILE")
fi

# Route based on tool type
if [[ "$TOOL_NAME" == "Bash" ]]; then
    # Extract Bash-specific fields
    if command -v jq &> /dev/null; then
        COMMAND=$(echo "$PARAMETERS" | jq -r '.command // empty')
        DESCRIPTION=$(echo "$PARAMETERS" | jq -r '.description // empty')
        EXIT_CODE=$(echo "$RESULT" | jq -r '.exit_code // null')
        STDOUT=$(echo "$RESULT" | jq -r '.stdout // empty')
        STDERR=$(echo "$RESULT" | jq -r '.stderr // empty')
    else
        COMMAND=""
        DESCRIPTION=""
        EXIT_CODE="null"
        STDOUT=""
        STDERR=""
    fi

    # Validate Bash fields
    if [[ -z "$COMMAND" ]]; then
        exit 0
    fi

    # Build JSON payload for shell command
    if command -v jq &> /dev/null; then
        PAYLOAD=$(jq -n \
            --arg session "$SESSION_ID" \
            --arg sessionName "$SESSION_NAME" \
            --arg command "$COMMAND" \
            --arg description "$DESCRIPTION" \
            --arg cwd "$CWD" \
            --arg branch "$GIT_BRANCH" \
            --arg modelProvider "$MODEL_PROVIDER" \
            --arg modelName "$MODEL_NAME" \
            --argjson exitCode "$EXIT_CODE" \
            --arg stdout "$STDOUT" \
            --arg stderr "$STDERR" \
            --argjson durationMs "$DURATION_MS" \
            '{
                session_id: $session,
                session_name: $sessionName,
                command: $command,
                description: $description,
                cwd: $cwd,
                branch: $branch,
                model_provider: $modelProvider,
                model_name: $modelName,
                exit_code: $exitCode,
                stdout: $stdout,
                stderr: $stderr,
                duration_ms: $durationMs
            }')
    else
        # Fallback: basic JSON (escape issues possible)
        PAYLOAD=$(cat <<EOF
{
  "session_id": "$SESSION_ID",
  "session_name": "$SESSION_NAME",
  "command": "$COMMAND",
  "description": "$DESCRIPTION",
  "cwd": "$CWD",
  "branch": "$GIT_BRANCH",
  "model_provider": "$MODEL_PROVIDER",
  "model_name": "$MODEL_NAME",
  "exit_code": $EXIT_CODE,
  "stdout": "$STDOUT",
  "stderr": "$STDERR",
  "duration_ms": $DURATION_MS
}
EOF
)
    fi

    # POST to shell endpoint
    if command -v curl &> /dev/null; then
        if [[ -n "$API_KEY" ]]; then
            curl -X POST "$SHELL_ENDPOINT" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $API_KEY" \
                -k \
                -d "$PAYLOAD" \
                &> /dev/null &
        else
            curl -X POST "$SHELL_ENDPOINT" \
                -H "Content-Type: application/json" \
                -k \
                -d "$PAYLOAD" \
                &> /dev/null &
        fi
    elif command -v wget &> /dev/null; then
        if [[ -n "$API_KEY" ]]; then
            wget --quiet --post-data="$PAYLOAD" \
                --header="Content-Type: application/json" \
                --header="Authorization: Bearer $API_KEY" \
                --no-check-certificate \
                -O /dev/null \
                "$SHELL_ENDPOINT" \
                &> /dev/null &
        else
            wget --quiet --post-data="$PAYLOAD" \
                --header="Content-Type: application/json" \
                --no-check-certificate \
                -O /dev/null \
                "$SHELL_ENDPOINT" \
                &> /dev/null &
        fi
    fi

else
    # Claude tool (Read, Edit, Write, etc.)

    # Convert success to boolean
    if [[ "$SUCCESS" == "true" ]] || [[ "$SUCCESS" == "1" ]]; then
        SUCCESS_BOOL=true
    else
        SUCCESS_BOOL=false
    fi

    # Build JSON payload for Claude command
    if command -v jq &> /dev/null; then
        PAYLOAD=$(jq -n \
            --arg session "$SESSION_ID" \
            --arg sessionName "$SESSION_NAME" \
            --arg toolName "$TOOL_NAME" \
            --argjson parameters "$PARAMETERS" \
            --argjson result "$RESULT" \
            --arg cwd "$CWD" \
            --arg branch "$GIT_BRANCH" \
            --arg modelProvider "$MODEL_PROVIDER" \
            --arg modelName "$MODEL_NAME" \
            --argjson success "$SUCCESS_BOOL" \
            --arg errorMessage "$ERROR_MESSAGE" \
            --argjson durationMs "$DURATION_MS" \
            '{
                session_id: $session,
                session_name: $sessionName,
                tool_name: $toolName,
                parameters: ($parameters | tostring),
                result: ($result | tostring),
                cwd: $cwd,
                branch: $branch,
                model_provider: $modelProvider,
                model_name: $modelName,
                success: $success,
                error_message: $errorMessage,
                duration_ms: $durationMs
            }')
    else
        # Fallback: basic JSON
        PAYLOAD=$(cat <<EOF
{
  "session_id": "$SESSION_ID",
  "session_name": "$SESSION_NAME",
  "tool_name": "$TOOL_NAME",
  "parameters": "$PARAMETERS",
  "result": "$RESULT",
  "cwd": "$CWD",
  "branch": "$GIT_BRANCH",
  "model_provider": "$MODEL_PROVIDER",
  "model_name": "$MODEL_NAME",
  "success": $SUCCESS_BOOL,
  "error_message": "$ERROR_MESSAGE",
  "duration_ms": $DURATION_MS
}
EOF
)
    fi

    # POST to Claude endpoint
    if command -v curl &> /dev/null; then
        if [[ -n "$API_KEY" ]]; then
            curl -X POST "$CLAUDE_ENDPOINT" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $API_KEY" \
                -k \
                -d "$PAYLOAD" \
                &> /dev/null &
        else
            curl -X POST "$CLAUDE_ENDPOINT" \
                -H "Content-Type: application/json" \
                -k \
                -d "$PAYLOAD" \
                &> /dev/null &
        fi
    elif command -v wget &> /dev/null; then
        if [[ -n "$API_KEY" ]]; then
            wget --quiet --post-data="$PAYLOAD" \
                --header="Content-Type: application/json" \
                --header="Authorization: Bearer $API_KEY" \
                --no-check-certificate \
                -O /dev/null \
                "$CLAUDE_ENDPOINT" \
                &> /dev/null &
        else
            wget --quiet --post-data="$PAYLOAD" \
                --header="Content-Type: application/json" \
                --no-check-certificate \
                -O /dev/null \
                "$CLAUDE_ENDPOINT" \
                &> /dev/null &
        fi
    fi
fi

# Exit successfully (don't block Claude Code)
exit 0
