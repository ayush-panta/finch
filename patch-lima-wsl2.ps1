# Patch Lima WSL2 driver to use accessible temp location

$limaFile = "deps/finch-core/src/lima/pkg/driver/wsl2/vm_windows.go"

# Backup original
Copy-Item $limaFile "$limaFile.backup"

# Replace temp file creation to use C:\ directly
$content = Get-Content $limaFile -Raw
$content = $content -replace 'os\.CreateTemp\("", "lima-wsl2-boot-\*\.sh"\)', 'os.CreateTemp("C:\\", "lima-wsl2-boot-*.sh")'

Set-Content $limaFile $content

Write-Host "Patched Lima to use C:\ for temp files" -ForegroundColor Green
Write-Host "Now run: make clean && make" -ForegroundColor Yellow