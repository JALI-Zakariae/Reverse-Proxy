Write-Host "Starting backend servers..." -ForegroundColor Cyan

$backend1 = Start-Process -FilePath "go" -ArgumentList "run", "backend.go", "9001" -PassThru
Write-Host "Backend 1 started on port 9001 (PID: $($backend1.Id))" -ForegroundColor Green

$backend2 = Start-Process -FilePath "go" -ArgumentList "run", "backend.go", "9002" -PassThru
Write-Host "Backend 2 started on port 9002 (PID: $($backend2.Id))" -ForegroundColor Green

$backend3 = Start-Process -FilePath "go" -ArgumentList "run", "backend.go", "9003" -PassThru
Write-Host "Backend 3 started on port 9003 (PID: $($backend3.Id))" -ForegroundColor Green

Write-Host ""
Write-Host "All backend servers started!" -ForegroundColor Cyan
Write-Host "Press Ctrl+C to stop all backends" -ForegroundColor Yellow
Write-Host ""

try {
    while ($true) {
        Start-Sleep -Seconds 1
    }
} finally {
    Write-Host "Stopping backend servers..." -ForegroundColor Yellow
    Stop-Process -Id $backend1.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $backend2.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $backend3.Id -Force -ErrorAction SilentlyContinue
    Write-Host "All backends stopped" -ForegroundColor Green
}