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
    local env_file="$2"
    
    # Read and export environment variables from JSON
    echo "ENV=$(jq -r '.env' "$config_file")" > "$env_file"
    echo "NODE_ENV=$(jq -r '.nodeEnv' "$config_file")" >> "$env_file"
    
    # UI config
    echo "UI_PORT=$(jq -r '.ui.port' "$config_file")" >> "$env_file"
    echo "VITE_WS_URL=$(jq -r '.ui.wsUrl' "$config_file")" >> "$env_file"
    echo "VITE_API_URL=$(jq -r '.ui.apiUrl' "$config_file")" >> "$env_file"
    
    # Server config
    echo "PORT=$(jq -r '.server.port' "$config_file")" >> "$env_file"
    echo "DB_PATH=$(jq -r '.server.dbPath' "$config_file")" >> "$env_file"
    
    # Nginx config
    echo "HTTP_PORT=$(jq -r '.nginx.httpPort' "$config_file")" >> "$env_file"
    echo "HTTPS_PORT=$(jq -r '.nginx.httpsPort' "$config_file")" >> "$env_file"
}

# Set environment variables based on the argument
case "$1" in
    "dev")
        echo "Switching to development environment..."
        set_env_from_config "$PROJECT_ROOT/config/dev.json" "$PROJECT_ROOT/.env"
        ;;
    "prod")
        echo "Switching to production environment..."
        set_env_from_config "$PROJECT_ROOT/config/prod.json" "$PROJECT_ROOT/.env"
        ;;
    *)
        show_usage
        exit 1
        ;;
esac

echo "Environment switched to $1"
echo "You can now run 'docker compose up' to start the services" 