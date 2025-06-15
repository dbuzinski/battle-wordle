#!/bin/bash

# Function to show usage
show_usage() {
    echo "Usage: $0 [ui|server|all]"
    echo "  ui     - Start UI in development mode"
    echo "  server - Start server in development mode"
    echo "  all    - Start both UI and server in development mode"
}

# Check if argument is provided
if [ $# -ne 1 ]; then
    show_usage
    exit 1
fi

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Set development environment
export ENV=development
export NODE_ENV=development
export UI_PORT=5173
export VITE_WS_URL=ws://localhost:8080/ws
export VITE_API_URL=http://localhost:8080/api
export PORT=8080
export DB_PATH=./battlewordle.db

case "$1" in
    "ui")
        echo "Starting UI in development mode..."
        cd "$PROJECT_ROOT/ui" && npm run dev
        ;;
    "server")
        echo "Starting server in development mode..."
        cd "$PROJECT_ROOT/server" && PROJECT_ROOT="$PROJECT_ROOT" go run cmd/main.go
        ;;
    "all")
        echo "Starting both UI and server in development mode..."
        # Start server in background
        cd "$PROJECT_ROOT/server" && PROJECT_ROOT="$PROJECT_ROOT" go run cmd/main.go &
        SERVER_PID=$!
        
        # Start UI
        cd "$PROJECT_ROOT/ui" && npm run dev
        
        # Cleanup server process when UI is stopped
        kill $SERVER_PID
        ;;
    *)
        show_usage
        exit 1
        ;;
esac 