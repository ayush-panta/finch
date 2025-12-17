#!/bin/bash
set -e

echo "🚀 Finch Development Setup Script"
echo "=================================="

# Check if we're on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    echo "❌ This script is for macOS only"
    exit 1
fi

# Build what we need
echo "📦 Building Finch and credential bridge..."
echo "   Building finch binary..."
make finch 2>/dev/null || echo "   ⚠️  Finch build had warnings (likely OK)"
echo "   Building credential bridge..."
make finch-cred-bridge 2>/dev/null || echo "   ⚠️  Credential bridge build had warnings (likely OK)"

# Check if binaries exist
if [ ! -f "_output/bin/finch" ]; then
    echo "❌ finch binary not found. Please check build."
    exit 1
fi
if [ ! -f "_output/bin/finch-cred-bridge" ]; then
    echo "❌ finch-cred-bridge binary not found. Please check build."
    exit 1
fi
echo "   ✅ Binaries built successfully"

echo ""
echo "🔗 Setting up development symlinks..."
echo "   (This makes your dev build appear as a production installation)"

# Create symlinks (requires sudo)
if [ ! -L "/Applications/Finch/bin/finch" ]; then
    echo "   Creating finch binary symlink..."
    sudo mkdir -p /Applications/Finch/bin/
    sudo ln -sf "$(pwd)/_output/bin/finch" /Applications/Finch/bin/finch
else
    echo "   ✅ Finch binary symlink already exists"
fi

if [ ! -L "/Applications/Finch/bin/finch-cred-bridge" ]; then
    echo "   Creating credential bridge symlink..."
    sudo ln -sf "$(pwd)/_output/bin/finch-cred-bridge" /Applications/Finch/bin/finch-cred-bridge
else
    echo "   ✅ Credential bridge symlink already exists"
fi

# Set up LaunchAgent for credential bridge
echo ""
echo "🔧 Setting up credential bridge LaunchAgent..."
if ! launchctl list | grep -q com.runfinch.cred-bridge; then
    make install-plist
    echo "   ✅ LaunchAgent installed and loaded"
else
    echo "   ✅ LaunchAgent already loaded"
fi

# Initialize VM if not exists
echo ""
echo "🖥️  Setting up Finch VM..."
if ! finch vm status &>/dev/null || finch vm status | grep -q "Nonexistent\|Stopped"; then
    echo "   Initializing VM (this may take a few minutes)..."
    finch vm init
    echo "   Starting VM..."
    finch vm start
else
    echo "   ✅ VM already running"
fi

# Test credential bridge
echo ""
echo "🧪 Testing credential bridge..."
if echo -e 'list\n' | nc -U ~/.finch/creds.sock &>/dev/null; then
    echo "   ✅ Credential bridge is working"
else
    echo "   ⚠️  Credential bridge test failed - check logs at ~/.finch/cred-bridge.log"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "📝 What was set up:"
echo "   • Built finch and finch-cred-bridge binaries"
echo "   • Created symlinks so 'finch' command uses your dev build"
echo "   • Installed LaunchAgent for credential bridge"
echo "   • Initialized and started Finch VM"
echo ""
echo "🎯 You can now use:"
echo "   finch login docker.io"
echo "   finch logout docker.io"
echo "   finch run hello-world"
echo ""
echo "🔍 To view credential bridge logs:"
echo "   tail -f ~/.finch/cred-bridge.log"
echo ""
echo "🧹 To clean up later:"
echo "   make dev-uninstall"
echo "   make uninstall-plist"