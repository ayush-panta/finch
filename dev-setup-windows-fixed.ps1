# Development setup script for finch credential helper testing on Windows
# Run this in PowerShell as Administrator

Write-Host "Checking git submodules..." -ForegroundColor Green
if (!(Test-Path "deps/finch-core/.git")) {
    Write-Host "   Initializing submodules..." -ForegroundColor Yellow
    git submodule update --init --recursive
} else {
    Write-Host "   Submodules already initialized" -ForegroundColor Green
}

Write-Host "Cleaning up previous builds..." -ForegroundColor Green
Remove-Item -Path "_output" -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "Setting Go environment..." -ForegroundColor Green
$env:GOSUMDB = ""

Write-Host "Building finch..." -ForegroundColor Green
make clean
make

# Check if build succeeded
if (!(Test-Path "_output/bin/finch.exe")) {
    Write-Host "Build failed - finch.exe not found" -ForegroundColor Red
    Write-Host "Try running: make -j1" -ForegroundColor Yellow
    exit 1
}

Write-Host "Build successful!" -ForegroundColor Green

# Check if credential helper was built
if (Test-Path "_output/bin/finch-credhelper.exe") {
    Write-Host "Setting up credential helper binary..." -ForegroundColor Green
    
    # Stop any running credential helper processes
    Get-Process -Name "finch-credhelper" -ErrorAction SilentlyContinue | Stop-Process -Force
    
    # Create credential helper directory
    $credHelperDir = "$env:LOCALAPPDATA\.finch\cred-helpers"
    New-Item -ItemType Directory -Path $credHelperDir -Force | Out-Null
    
    # Copy credential helper binary
    Copy-Item -Path "_output/bin/finch-credhelper.exe" -Destination "$credHelperDir\finch-credhelper.exe" -Force
    
    Write-Host "Credential helper binary located at: $credHelperDir\finch-credhelper.exe" -ForegroundColor Yellow
} else {
    Write-Host "Credential helper not built - skipping setup" -ForegroundColor Yellow
}

Write-Host "Initializing VM (this may take a few minutes)..." -ForegroundColor Green
$initResult = Start-Process -FilePath "./_output/bin/finch.exe" -ArgumentList "vm", "init" -Wait -PassThru
if ($initResult.ExitCode -eq 0) {
    Write-Host "VM initialized successfully!" -ForegroundColor Green
} else {
    Write-Host "VM initialization failed" -ForegroundColor Red
    exit 1
}

Write-Host "Checking VM status..." -ForegroundColor Green
Start-Process -FilePath "./_output/bin/finch.exe" -ArgumentList "vm", "status" -Wait -NoNewWindow

Write-Host "Setup complete!" -ForegroundColor Green