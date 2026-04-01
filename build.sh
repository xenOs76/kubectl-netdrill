#!/usr/bin/env bash

# Exit on error
set -e

# Create dist directory
mkdir -p dist

# Build the plugin
echo "Building kubectl-netdrill..."
go build -o dist/kubectl-netdrill main.go

echo "Build complete. Binary is at dist/kubectl-netdrill"
