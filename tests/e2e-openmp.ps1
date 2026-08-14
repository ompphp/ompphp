$ErrorActionPreference = 'Stop'

$workflowRun = if ($env:OPENMP_WORKFLOW_RUN) { $env:OPENMP_WORKFLOW_RUN } else { '31335425385' }
$artifactSha256 = if ($env:OPENMP_ARTIFACT_SHA256) { $env:OPENMP_ARTIFACT_SHA256 } else { 'bb7bd2dd846b0af9b0374858f4d092ce76e7106ca0d9df518ee8916e9f83ca74' }
$artifactUrl = "https://nightly.link/openmultiplayer/open.mp/actions/runs/$workflowRun/open.mp-win-x64-.zip"
$testDir = Join-Path ([System.IO.Path]::GetTempPath()) ("ompphp-e2e-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $testDir | Out-Null

try {
    Write-Host 'Downloading pinned open.mp Windows x64 artifact'
    Invoke-WebRequest -Uri $artifactUrl -OutFile (Join-Path $testDir 'artifact.zip') -TimeoutSec 180
    $actualSha256 = (Get-FileHash (Join-Path $testDir 'artifact.zip') -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualSha256 -ne $artifactSha256.ToLowerInvariant()) {
        throw "open.mp artifact checksum mismatch: expected $artifactSha256, got $actualSha256"
    }
    Expand-Archive (Join-Path $testDir 'artifact.zip') -DestinationPath $testDir
    Expand-Archive (Join-Path $testDir 'open.mp-win-x64-.zip') -DestinationPath $testDir

    $serverDir = Join-Path $testDir 'Server'
    Copy-Item (Join-Path $PSScriptRoot '../build/ompphp.dll') (Join-Path $serverDir 'components/ompphp.dll')
    Copy-Item (Join-Path $PSScriptRoot 'fixtures/e2e/gamemode.php') (Join-Path $serverDir 'gamemode.php')
    Copy-Item (Join-Path $PSScriptRoot 'fixtures/e2e/composer.json') (Join-Path $serverDir 'composer.json')
    $packagesDir = Join-Path $serverDir 'packages'
    New-Item -ItemType Directory -Path $packagesDir | Out-Null
    php -r "exit(extension_loaded('zip') ? 0 : 1);"
    if ($LASTEXITCODE -ne 0) {
        throw "PHP zip extension is required for the SDK artifact repository"
    }
    Push-Location (Join-Path $PSScriptRoot '..')
    try {
        go run ./tools/sdkpack -version 0.1.0-beta.1 -out $packagesDir
        if ($LASTEXITCODE -ne 0) {
            throw "SDK packaging failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
    composer install --working-dir=$serverDir --no-dev --no-interaction --no-progress
    if ($LASTEXITCODE -ne 0) {
        throw "Composer install failed with exit code $LASTEXITCODE"
    }

    $stdout = Join-Path $testDir 'stdout.log'
    $stderr = Join-Path $testDir 'stderr.log'
    $env:OMPPHP_ENTRY = 'gamemode.php'
    $process = Start-Process -FilePath (Join-Path $serverDir 'omp-server.exe') -WorkingDirectory $serverDir -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
    Start-Sleep -Seconds 5
    if (!$process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
    }

    $log = ((Get-Content -Raw $stdout), (Get-Content -Raw $stderr)) -join "`n"
    foreach ($marker in @(
        'Successfully loaded component ompphp',
        'OMPPHP_E2E_READY',
        'OMPPHP_E2E_SDK',
        'PHP handler for Tick failed:',
        'RuntimeException: OMPPHP_E2E_EXPECTED_FAILURE',
        'Stack trace:',
        'OMPPHP_E2E_TICK'
    )) {
        if (!$log.Contains($marker)) {
            Write-Host $log
            throw "Missing server marker: $marker"
        }
        Write-Host $marker
    }
} finally {
    Remove-Item -Recurse -Force $testDir -ErrorAction SilentlyContinue
}
