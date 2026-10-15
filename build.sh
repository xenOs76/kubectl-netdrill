#!/usr/bin/env bash

# Exit on error
set -e

# Create dist directory
mkdir -p dist

# Build the plugin
echo "Building kubectl-netdrill..."
go build -o dist/kubectl-netdrill main.go
ln -sf kubectl-netdrill dist/kubectl_complete-netdrill

echo "Build complete. Binaries are at dist/kubectl-netdrill and dist/kubectl_complete-netdrill"
