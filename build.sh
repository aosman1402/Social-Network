#!/bin/sh
# Builds the backend and frontend images and starts both containers.
# Usage: ./build.sh

set -e
cd "$(dirname "$0")"

if ! docker info >/dev/null 2>&1; then
  echo "Docker isn't running - start Docker Desktop (or 'colima start') and run ./build.sh again."
  exit 1
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
else
  COMPOSE="docker-compose"
fi

echo "Building images and starting containers..."
$COMPOSE up -d --build

echo
docker ps -a --size --filter "name=social-network" --format 'table {{.Names}}\t{{.Status}}\t{{.Size}}'

echo
echo "App: http://localhost:3000"
echo "API: http://localhost:8081/api/health"
echo "Stop everything with: $COMPOSE down"
