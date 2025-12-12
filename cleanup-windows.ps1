# Clean up corrupted Finch VM state on Windows

Write-Host "Cleaning up corrupted Finch VM state..." -ForegroundColor Yellow

# Stop any running processes
Write-Host "Stopping Finch processes..." -ForegroundColor Green
Get-Process -Name "*finch*" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "*lima*" -ErrorAction SilentlyContinue | Stop-Process -Force

# Remove Lima data directories
Write-Host "Removing Lima data directories..." -ForegroundColor Green
$limaDataPaths = @(
    "$env:USERPROFILE\.lima",
    "$env:LOCALAPPDATA\.finch",
    "C:\Users\Administrator\Documents\finch\_output\lima"
)

foreach ($path in $limaDataPaths) {
    if (Test-Path $path) {
        Write-Host "  Removing: $path" -ForegroundColor Cyan
        Remove-Item -Path $path -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# Clean WSL distributions
Write-Host "Cleaning WSL distributions..." -ForegroundColor Green
$wslDistros = wsl --list --quiet 2>$null | Where-Object { $_ -match "finch|lima" }
foreach ($distro in $wslDistros) {
    if ($distro.Trim()) {
        Write-Host "  Unregistering WSL distro: $($distro.Trim())" -ForegroundColor Cyan
        wsl --unregister $distro.Trim() 2>$null
    }
}

Write-Host "Cleanup complete! You can now run vm init." -ForegroundColor Green