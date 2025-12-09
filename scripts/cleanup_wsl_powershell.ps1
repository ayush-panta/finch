# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

# We want these cleanup commands to always run, ignore errors so the step completes.
$ErrorActionPreference = 'SilentlyContinue'

# Kill wslservice.exe if running
Get-Process -Name "wslservice" -ErrorAction SilentlyContinue | Stop-Process -Force

# Start LxssManager if stopped
$service = Get-Service -Name "LxssManager" -ErrorAction SilentlyContinue
if ($service -and $service.Status -eq "Stopped") {
    Start-Service -Name "LxssManager"
}

# List current WSL distributions
Write-Host "Current WSL distributions:"
wsl --list --verbose --all

# Attempt to shut down WSL if any distribution is running
$runningDistros = wsl --list --verbose | Select-String "Running"
if ($runningDistros) {
    Write-Host "Shutting down WSL..."
    wsl --shutdown
    Start-Sleep -Seconds 5
    
    # Forcefully kill WSL processes if still running
    Get-Process -Name "wsl*" -ErrorAction SilentlyContinue | Stop-Process -Force
    Write-Host "WSL has been shut down successfully."
}

# Unregister 'lima-finch' distribution if it exists
$distributions = wsl --list --quiet
if ($distributions -match "lima-finch") {
    Write-Host "Unregistering lima-finch distribution..."
    wsl --unregister lima-finch
    Write-Host "'lima-finch' has been unregistered successfully."
}

# List distributions after cleanup
Write-Host "WSL distributions after cleanup:"
wsl --list --verbose --all

# Remove Finch directories
Write-Host "Removing Finch directories..."
Remove-Item -Path "C:\Users\Administrator\AppData\Local\.finch" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path "C:\Users\Administrator\.finch" -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "WSL cleanup completed."