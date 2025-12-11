#!/bin/bash

# Development setup script for finch credential helper testing
set -e

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

echo "🔄 Reloading credential helper service..."
launchctl unload ~/Library/LaunchAgents/com.runfinch.cred-bridge.plist 2>/dev/null || true
cp cmd/finch-credhelper/com.runfinch.cred-bridge.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.runfinch.cred-bridge.plist

echo "🖥️  Initializing VM..."
./_output/bin/finch vm init

echo "✅ Setup complete!"
echo "📝 Credential helper will be managed by launchd"
echo "🔍 To view logs: tail -f _output/finch-credhelper/cred-bridge.log"