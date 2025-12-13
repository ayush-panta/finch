# Fix WSL2 initialization issues for Finch

Write-Host "Fixing WSL2 initialization issues..." -ForegroundColor Yellow

# Ensure WSL is properly configured
Write-Host "Checking WSL configuration..." -ForegroundColor Green
wsl --set-default-version 2

# Clean up any existing lima-finch distro
Write-Host "Cleaning up existing WSL distros..." -ForegroundColor Green
$existingDistros = wsl --list --quiet 2>$null | Where-Object { $_ -match "lima-finch" }
foreach ($distro in $existingDistros) {
    if ($distro.Trim()) {
        Write-Host "  Unregistering: $($distro.Trim())" -ForegroundColor Cyan
        wsl --unregister $distro.Trim() 2>$null
    }
}

# Set proper temp directory permissions
Write-Host "Setting temp directory permissions..." -ForegroundColor Green
$tempDir = $env:TEMP
if (Test-Path $tempDir) {
    # Grant full control to current user
    icacls $tempDir /grant "${env:USERNAME}:(OI)(CI)F" /T 2>$null
}

# Create a custom temp directory for Lima
Write-Host "Creating Lima temp directory..." -ForegroundColor Green
$limaTemp = "C:\lima-temp"
if (!(Test-Path $limaTemp)) {
    New-Item -ItemType Directory -Path $limaTemp -Force | Out-Null
}
icacls $limaTemp /grant "${env:USERNAME}:(OI)(CI)F" /T 2>$null

# Set environment variable for Lima to use our temp directory
$env:TMPDIR = $limaTemp
$env:TMP = $limaTemp
$env:TEMP = $limaTemp

Write-Host "WSL2 environment prepared. Try vm init again." -ForegroundColor Green
Write-Host "If it still fails, try running as Administrator." -ForegroundColor Yellow