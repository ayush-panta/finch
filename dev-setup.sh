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

echo "💀 Killing existing credential servers..."
pkill -f "finch-cred-server" || true

echo "🚀 Starting credential server in background..."
nohup ./_output/bin/finch-cred-server > cred-server.log 2>&1 &
CRED_SERVER_PID=$!
echo "Credential server started with PID: $CRED_SERVER_PID"
echo "Server logs: tail -f cred-server.log"

# Give the server a moment to start
sleep 2

echo "🖥️  Initializing VM..."
./_output/bin/finch vm init

echo "✅ Setup complete!"
echo "📝 Credential server PID: $CRED_SERVER_PID"
echo "🔍 To test connection from VM: echo 'test' | nc 192.168.5.2 8080"
echo "🛑 To stop server: kill $CRED_SERVER_PID"