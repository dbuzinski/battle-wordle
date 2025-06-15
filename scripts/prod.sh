#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Set production environment
source "$SCRIPT_DIR/set-env.sh" prod

# Stop any existing containers
cd "$PROJECT_ROOT" && docker compose --env-file .env down

# Start the services
cd "$PROJECT_ROOT" && docker compose --env-file .env up --build -d