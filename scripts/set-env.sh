#!/bin/bash

# Function to show usage
show_usage() {
    echo "Usage: $0 [dev|prod]"
    echo "  dev  - Switch to development environment (direct run)"
    echo "  prod - Switch to production environment (Docker)"
}

# Check if environment argument is provided
if [ $# -ne 1 ]; then
    show_usage
    exit 1
fi

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Function to read JSON config and export environment variables
set_env_from_config() {
    local config_file="$1"
    
    # Read and export environment variables from JSON
    export ENV=$(jq -r '.env' "$config_file")
    export NODE_ENV=$(jq -r '.nodeEnv' "$config_file")
    
    # UI config
    export UI_PORT=$(jq -r '.ui.port' "$config_file")
    export VITE_WS_URL=$(jq -r '.ui.wsUrl' "$config_file")
    export VITE_API_URL=$(jq -r '.ui.apiUrl' "$config_file")
    
    # Server config
    export PORT=$(jq -r '.server.port' "$config_file")
    export DB_PATH=$(jq -r '.server.dbPath' "$config_file")
    
    # Nginx config
    export HTTP_PORT=$(jq -r '.nginx.httpPort' "$config_file")
    export HTTPS_PORT=$(jq -r '.nginx.httpsPort' "$config_file")
}

# Set environment variables based on the argument
case "$1" in
    "dev")
        echo "Switching to development environment..."
        set_env_from_config "$PROJECT_ROOT/config/dev.json"
        ;;
    "prod")
        echo "Switching to production environment..."
        set_env_from_config "$PROJECT_ROOT/config/prod.json"
        # Use production nginx config
        cp "$PROJECT_ROOT/nginx/nginx.conf" "$PROJECT_ROOT/nginx/nginx.conf"
        ;;
    *)
        show_usage
        exit 1
        ;;
esac

echo "Environment switched to $1"
echo "You can now run 'docker compose up' to start the services" 