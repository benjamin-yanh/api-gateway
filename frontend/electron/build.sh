#!/bin/bash

set -e

echo "Building New API Electron App..."

echo "Step 1: Building frontend..."
cd ..
bun install --frozen-lockfile
DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(git describe --tags --always) bun run build
cd electron

echo "Step 2: Building Go backend..."
mkdir -p ../../backend/frontend
cp -R ../dist ../../backend/frontend/
cd ../../backend

if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "Building for macOS..."
    CGO_ENABLED=1 go build -tags embed -ldflags="-s -w" -o ../new-api
    cd ../frontend/electron
    npm install
    npm run build:mac
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "Building for Linux..."
    CGO_ENABLED=1 go build -tags embed -ldflags="-s -w" -o ../new-api
    cd ../frontend/electron
    npm install
    npm run build:linux
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
    echo "Building for Windows..."
    CGO_ENABLED=1 go build -tags embed -ldflags="-s -w" -o ../new-api.exe
    cd ../frontend/electron
    npm install
    npm run build:win
else
    echo "Unknown OS, building for current platform..."
    CGO_ENABLED=1 go build -tags embed -ldflags="-s -w" -o ../new-api
    cd ../frontend/electron
    npm install
    npm run build
fi

echo "Build complete! Check frontend/electron/dist/ for output."
