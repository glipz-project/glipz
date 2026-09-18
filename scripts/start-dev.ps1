$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
if (-not (Test-Path -LiteralPath '.env')) {
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $bytes = New-Object byte[] 48
        $rng.GetBytes($bytes)
        $jwt = [Convert]::ToBase64String($bytes)
        $rng.GetBytes($bytes)
        $seed = [Convert]::ToBase64String($bytes)
    } finally { $rng.Dispose() }
    @"
JWT_SECRET=$jwt
GLIPZ_STORAGE_MODE=local
GLIPZ_LOCAL_STORAGE_PATH=/app/data/media
FRONTEND_ORIGIN=http://127.0.0.1:8080
GLIPZ_MEDIA_PROXY_MODE=proxy
GLIPZ_AUTH_RATE_LIMIT_FAIL_CLOSED=true
GLIPZ_FEDERATION_KEY_SEED=$seed
PATREON_ENABLED=false
"@ | Set-Content -LiteralPath '.env' -Encoding ascii
}
New-Item -ItemType Directory -Force -Path 'data/media', 'data/legal-docs' | Out-Null
docker compose up -d --build --wait --wait-timeout 180
if ($LASTEXITCODE -ne 0) { throw 'Docker Compose startup failed.' }
Write-Host 'Glipz: http://127.0.0.1:8080'
Write-Host 'Mailpit: http://127.0.0.1:8025'
