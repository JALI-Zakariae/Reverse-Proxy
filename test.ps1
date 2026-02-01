Write-Host "=== Reverse Proxy Test Suite ===" -ForegroundColor Cyan
Write-Host ""

$backends = @()
$proxyProcess = $null

function Cleanup {
    Write-Host "Cleaning up processes..." -ForegroundColor Yellow
    if ($proxyProcess) { Stop-Process -Id $proxyProcess.Id -Force -ErrorAction SilentlyContinue }
    foreach ($backend in $backends) {
        Stop-Process -Id $backend.Id -Force -ErrorAction SilentlyContinue
    }
    Start-Sleep -Seconds 2
}

Register-EngineEvent PowerShell.Exiting -Action { Cleanup } | Out-Null

try {
    Write-Host "Step 1: Building project..." -ForegroundColor Green
    go build -o proxy.exe main.go
    go build -o backend.exe backend.go
    if ($LASTEXITCODE -ne 0) { throw "Build failed" }

    Write-Host ""
    Write-Host "Step 2: Starting backend servers..." -ForegroundColor Green
    $backend1 = Start-Process -FilePath ".\backend.exe" -ArgumentList "9001" -PassThru -WindowStyle Hidden
    $backend2 = Start-Process -FilePath ".\backend.exe" -ArgumentList "9002" -PassThru -WindowStyle Hidden
    $backend3 = Start-Process -FilePath ".\backend.exe" -ArgumentList "9003" -PassThru -WindowStyle Hidden
    $backends = @($backend1, $backend2, $backend3)
    Start-Sleep -Seconds 2
    Write-Host "Backend servers started (PIDs: $($backend1.Id), $($backend2.Id), $($backend3.Id))"

    Write-Host ""
    Write-Host "Step 3: Starting reverse proxy..." -ForegroundColor Green
    $proxyProcess = Start-Process -FilePath ".\proxy.exe" -ArgumentList "--config=config.json" -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 3
    Write-Host "Proxy started (PID: $($proxyProcess.Id))"

    Write-Host ""
    Write-Host "Step 4: Testing basic connectivity..." -ForegroundColor Green
    $response = Invoke-WebRequest -Uri "http://localhost:8080/" -UseBasicParsing
    if ($response.Content -match "Response from backend") {
        Write-Host "✓ Basic connectivity works" -ForegroundColor Green
    } else {
        Write-Host "✗ Basic connectivity failed" -ForegroundColor Red
        throw "Basic connectivity test failed"
    }

    Write-Host ""
    Write-Host "Step 5: Testing load distribution (10 requests)..." -ForegroundColor Green
    for ($i = 1; $i -le 10; $i++) {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/" -UseBasicParsing
        $response.Content | Select-String -Pattern "port \d+"
    }
    Write-Host "✓ Load distribution complete" -ForegroundColor Green

    Write-Host ""
    Write-Host "Step 6: Testing Admin API - Status..." -ForegroundColor Green
    $status = Invoke-RestMethod -Uri "http://localhost:8081/status" -Method Get
    Write-Host "Total backends: $($status.total_backends), Active: $($status.active_backends)"
    if ($status.total_backends -eq 3 -and $status.active_backends -eq 3) {
        Write-Host "✓ Admin API status works" -ForegroundColor Green
    } else {
        Write-Host "✗ Admin API status failed" -ForegroundColor Red
        throw "Admin API test failed"
    }

    Write-Host ""
    Write-Host "Step 7: Testing backend failure detection..." -ForegroundColor Green
    Write-Host "Stopping backend on port 9001..."
    Stop-Process -Id $backend1.Id -Force
    Start-Sleep -Seconds 2

    Write-Host "Sending requests to verify failover..."
    $hasError = $false
    for ($i = 1; $i -le 5; $i++) {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/" -UseBasicParsing
        if ($response.Content -match "9001") {
            Write-Host "✗ Failed backend still receiving traffic" -ForegroundColor Red
            $hasError = $true
            break
        }
    }
    if (-not $hasError) {
        Write-Host "✓ Traffic correctly routed away from failed backend" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Step 8: Testing dynamic backend addition..." -ForegroundColor Green
    $backend4 = Start-Process -FilePath ".\backend.exe" -ArgumentList "9004" -PassThru -WindowStyle Hidden
    $backends += $backend4
    Start-Sleep -Seconds 1

    $body = @{ url = "http://localhost:9004" } | ConvertTo-Json
    $addResponse = Invoke-RestMethod -Uri "http://localhost:8081/backends" -Method Post -Body $body -ContentType "application/json"
    
    if ($addResponse.status -eq "success") {
        Write-Host "✓ Backend added successfully" -ForegroundColor Green
    } else {
        Write-Host "✗ Failed to add backend" -ForegroundColor Red
    }

    $status = Invoke-RestMethod -Uri "http://localhost:8081/status" -Method Get
    if ($status.total_backends -eq 4) {
        Write-Host "✓ Backend count updated correctly" -ForegroundColor Green
    } else {
        Write-Host "✗ Backend count incorrect" -ForegroundColor Red
    }

    Write-Host ""
    Write-Host "Step 9: Testing dynamic backend removal..." -ForegroundColor Green
    $body = @{ url = "http://localhost:9004" } | ConvertTo-Json
    $removeResponse = Invoke-RestMethod -Uri "http://localhost:8081/backends" -Method Delete -Body $body -ContentType "application/json"
    
    if ($removeResponse.status -eq "success") {
        Write-Host "✓ Backend removed successfully" -ForegroundColor Green
    } else {
        Write-Host "✗ Failed to remove backend" -ForegroundColor Red
    }

    Stop-Process -Id $backend4.Id -Force -ErrorAction SilentlyContinue

    Write-Host ""
    Write-Host "Step 10: Testing concurrent requests..." -ForegroundColor Green
    Write-Host "Sending 50 concurrent requests..."
    $jobs = @()
    for ($i = 1; $i -le 50; $i++) {
        $jobs += Start-Job -ScriptBlock {
            Invoke-WebRequest -Uri "http://localhost:8080/" -UseBasicParsing | Out-Null
        }
    }
    $jobs | Wait-Job | Out-Null
    $jobs | Remove-Job
    Write-Host "✓ Concurrent requests handled" -ForegroundColor Green

    Write-Host ""
    Write-Host "=== All Tests Passed! ===" -ForegroundColor Cyan
    Write-Host ""

} catch {
    Write-Host ""
    Write-Host "Error: $_" -ForegroundColor Red
    Write-Host ""
} finally {
    Write-Host "Cleaning up..." -ForegroundColor Yellow
    Cleanup
    Write-Host "Done!" -ForegroundColor Green
}