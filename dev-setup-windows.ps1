# Development setup script for finch credential helper testing on Windows
# Run this in PowerShell as Administrator

Write-Host "📦 Initializing git submodules..." -ForegroundColor Green
git submodule update --init --recursive

Write-Host "🧹 Cleaning up previous builds..." -ForegroundColor Green
Remove-Item -Path "_output" -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "🔧 Setting Go environment..." -ForegroundColor Green
$env:GOSUMDB = ""

Write-Host "🧽 Running make clean..." -ForegroundColor Green
make clean

Write-Host "🔨 Building finch..." -ForegroundColor Green
make

Write-Host "🧹 Cleaning credential helper log..." -ForegroundColor Green
Remove-Item -Path "_output/finch-credhelper/cred-bridge.log" -Force -ErrorAction SilentlyContinue

Write-Host "🔄 Setting up credential helper binary..." -ForegroundColor Green
# Stop any running credential helper processes
Get-Process -Name "finch-credhelper" -ErrorAction SilentlyContinue | Stop-Process -Force

# Create credential helper directory if it doesn't exist
$credHelperDir = "$env:LOCALAPPDATA\.finch\cred-helpers"
New-Item -ItemType Directory -Path $credHelperDir -Force | Out-Null

# Copy credential helper binary
Copy-Item -Path "_output/bin/finch-credhelper.exe" -Destination "$credHelperDir\finch-credhelper.exe" -Force

Write-Host "🖥️  Initializing VM..." -ForegroundColor Green
& ".\_output\bin\finch.exe" vm init

Write-Host "✅ Setup complete!" -ForegroundColor Green
Write-Host "📝 Credential helper binary located at: $credHelperDir\finch-credhelper.exe" -ForegroundColor Yellow
Write-Host "🔧 To run credential helper manually:" -ForegroundColor Yellow
Write-Host "   `& '$credHelperDir\finch-credhelper.exe'" -ForegroundColor Cyan
Write-Host "🔍 To view logs: Get-Content _output\finch-credhelper\cred-bridge.log -Wait" -ForegroundColor Yellow