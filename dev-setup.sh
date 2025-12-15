#!/bin/bash

# Development setup script for finch credential helper testing
set -e

echo "📦 Initializing git submodules..."
git submodule update --init --recursive

echo "🧹 Cleaning up previous builds..."
rm -rf _output

echo "🔧 Setting Go environment..."
unset GOSUMDB

echo "🧽 Running make clean..."
make clean

echo "🔨 Building finch..."
make

echo "🧹 Cleaning credential helper log..."
rm -f _output/finch-credhelper/cred-bridge.log

echo "🖥️  Initializing VM..."
./_output/bin/finch vm init

echo "✅ Setup complete!"
echo "📝 Credential helper will be managed by launchd"
echo "🔍 To view logs: tail -f _output/finch-credhelper/cred-bridge.log"