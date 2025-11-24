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
rm -f ~/.finch/cred-helper.log

echo "🔄 Reloading credential helper service..."
launchctl unload ~/Library/LaunchAgents/com.runfinch.credhelper.plist 2>/dev/null || true
launchctl load ~/Library/LaunchAgents/com.runfinch.credhelper.plist

echo "🖥️  Initializing VM..."
./_output/bin/finch vm init

echo "✅ Setup complete!"
echo "📝 Credential helper will be managed by launchd"
echo "🔍 To view logs: tail -f ~/Documents/finch-creds/finch/cred-helper.log"
echo "🧪 To test socket: echo -e 'erase\nhttps://index.docker.io/v1/' | nc -U ~/.finch/creds.sock"