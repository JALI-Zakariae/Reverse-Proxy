Write-Host "Stopping all reverse proxy processes..." -ForegroundColor Yellow

Get-Process | Where-Object { $_.ProcessName -eq "proxy" } | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process | Where-Object { $_.ProcessName -eq "backend" } | Stop-Process -Force -ErrorAction SilentlyContinue

Get-Process | Where-Object { $_.CommandLine -like "*backend.go*" } | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process | Where-Object { $_.CommandLine -like "*main.go*" } | Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "All processes stopped" -ForegroundColor Green