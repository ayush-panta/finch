# Development setup script for finch credential helper testing on Windows
$ErrorActionPreference = "Stop"

Write-Host "🚀 Finch Windows Development Setup Script" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Write-Host "📦 Cleaning up build artifacts..." -ForegroundColor Yellow
make clean
cd deps/finch-core
make clean
cd ../..

Write-Host "📦 Syncing submodules..." -ForegroundColor Yellow
git submodule update --init --recursive

Write-Host "🔨 Building everything with make..." -ForegroundColor Yellow
make

# Rename bridge to add .exe extension
if (Test-Path "./_output/bin/finch-cred-bridge") {
    if (Test-Path "./_output/bin/finch-cred-bridge.exe") {
        Remove-Item "./_output/bin/finch-cred-bridge.exe" -Force
    }
    Rename-Item "./_output/bin/finch-cred-bridge" -NewName "finch-cred-bridge.exe"
}

# Write-Host "🧹 Cleaning credential helper log..." -ForegroundColor Yellow
# New-Item -ItemType Directory -Force -Path "C:\temp" | Out-Null
# Remove-Item -Force -ErrorAction SilentlyContinue "$env:USERPROFILE\.finch\cred-helper.log"

# Write-Host "🔄 Setting up Windows credential bridge service..." -ForegroundColor Yellow
# & "./_output/bin/finch-cred-bridge.exe" -install
# & "./_output/bin/finch-cred-bridge.exe" -start

Write-Host "🌐 Configuring WSL networking..." -ForegroundColor Yellow
$wslConfig = "[experimental]`nnetworkingMode=mirrored`nhostAddressLoopback=true"
Set-Content -Path "C:\Users\Administrator\.wslconfig" -Value $wslConfig -Encoding UTF8

Write-Host "🧽 Cleaning up WSL and VM state..." -ForegroundColor Yellow
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue "C:\Users\Administrator\.finch"
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue "C:\Users\Administrator\AppData\Local\.finch"
./scripts/cleanup_wsl.ps1

Write-Host "🖥️  Initializing VM..." -ForegroundColor Yellow
& "./_output/bin/finch.exe" vm init

Write-Host "✅ Setup complete!" -ForegroundColor Green
# Write-Host "📝 Credential helper will be managed by Windows services" -ForegroundColor Cyan
# Write-Host "🔍 To view logs: Get-Content $env:USERPROFILE\.finch\cred-helper.log -Wait" -ForegroundColor Cyan