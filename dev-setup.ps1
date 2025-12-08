# Development setup script for finch credential helper testing on Windows
$ErrorActionPreference = "Stop"

Write-Host "🚀 Finch Windows Development Setup Script" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Write-Host "📦 Syncing submodules..." -ForegroundColor Yellow
git submodule update --init --recursive

Write-Host "🔨 Building everything with make..." -ForegroundColor Yellow
make

# Rename bridge to add .exe extension
if (Test-Path "./_output/bin/finch-cred-bridge") {
    Rename-Item "./_output/bin/finch-cred-bridge" -NewName "finch-cred-bridge.exe"
}

# Write-Host "🧹 Cleaning credential helper log..." -ForegroundColor Yellow
# New-Item -ItemType Directory -Force -Path "C:\temp" | Out-Null
# Remove-Item -Force -ErrorAction SilentlyContinue "$env:USERPROFILE\.finch\cred-helper.log"

# Write-Host "🔄 Setting up Windows credential bridge service..." -ForegroundColor Yellow
# & "./_output/bin/finch-cred-bridge.exe" -install
# & "./_output/bin/finch-cred-bridge.exe" -start

Write-Host "🖥️  Initializing VM..." -ForegroundColor Yellow
& "./_output/bin/finch.exe" vm init

Write-Host "✅ Setup complete!" -ForegroundColor Green
# Write-Host "📝 Credential helper will be managed by Windows services" -ForegroundColor Cyan
# Write-Host "🔍 To view logs: Get-Content $env:USERPROFILE\.finch\cred-helper.log -Wait" -ForegroundColor Cyan