#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
source "$PROJECT_ROOT/.env"

echo "Starting development server..."
# Start server in background
cd "$PROJECT_ROOT/server" && PROJECT_ROOT="$PROJECT_ROOT" go run main.go &
SERVER_PID=$!

# Start UI
cd "$PROJECT_ROOT/ui" && npm run dev

# Cleanup server process when UI is stopped
kill $SERVER_PID