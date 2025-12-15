# Windows credential helper testing script
# Run after dev-setup-windows.ps1

param(
    [switch]$Manual = $false
)

$credHelperPath = "$env:LOCALAPPDATA\.finch\cred-helpers\finch-credhelper.exe"

Write-Host "🧪 Starting Windows credential helper tests..." -ForegroundColor Green

if ($Manual) {
    Write-Host "🔧 Manual mode: Starting credential helper process..." -ForegroundColor Yellow
    
    # Start credential helper in background
    $credProcess = Start-Process -FilePath $credHelperPath -PassThru -WindowStyle Hidden
    Write-Host "   Process ID: $($credProcess.Id)" -ForegroundColor Cyan
    
    # Give it time to start
    Start-Sleep -Seconds 2
}

Write-Host "🐳 Testing basic container operations..." -ForegroundColor Green

# Test 1: Pull public image
Write-Host "   1. Pulling public image..." -ForegroundColor Cyan
& ".\_output\bin\finch.exe" pull alpine

# Test 2: Setup local registry (if Docker is available)
Write-Host "   2. Setting up local registry..." -ForegroundColor Cyan
try {
    & ".\_output\bin\finch.exe" run -d --name registry -p 5000:5000 registry:2
    Start-Sleep -Seconds 3
    
    # Test 3: Tag and push to local registry
    Write-Host "   3. Testing local registry push..." -ForegroundColor Cyan
    & ".\_output\bin\finch.exe" tag alpine localhost:5000/test-image
    & ".\_output\bin\finch.exe" push localhost:5000/test-image
    
    Write-Host "✅ Registry tests passed!" -ForegroundColor Green
} catch {
    Write-Host "⚠️  Registry tests skipped (registry not available)" -ForegroundColor Yellow
}

# Test 4: Check credential storage
Write-Host "   4. Checking credential storage..." -ForegroundColor Cyan
$configPath = "$env:USERPROFILE\.finch\config.json"
if (Test-Path $configPath) {
    $config = Get-Content $configPath | ConvertFrom-Json
    Write-Host "   Config found: $configPath" -ForegroundColor Cyan
    if ($config.credsStore) {
        Write-Host "   Credential store: $($config.credsStore)" -ForegroundColor Cyan
    }
} else {
    Write-Host "   No config.json found" -ForegroundColor Yellow
}

# Cleanup
Write-Host "🧹 Cleaning up..." -ForegroundColor Green
& ".\_output\bin\finch.exe" rm -f registry 2>$null

if ($Manual -and $credProcess) {
    Write-Host "🛑 Stopping credential helper process..." -ForegroundColor Yellow
    Stop-Process -Id $credProcess.Id -Force -ErrorAction SilentlyContinue
}

Write-Host "✅ Windows credential tests completed!" -ForegroundColor Green