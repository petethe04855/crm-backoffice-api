# Starts backoffice-api for local development.
# Asks for the secrets at hidden prompts (unless already set in the environment
# or backoffice-api/.env), keeps them only in this PowerShell process, then runs
# `go run ./cmd/api`. Nothing is written to disk.

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

function Read-Secret([string]$prompt) {
    $secure = Read-Host -Prompt $prompt -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
}

if (-not $env:CRM_DB_PASSWORD) {
    $env:CRM_DB_PASSWORD = Read-Secret 'DB_PASSWORD (same as DB_PASSWORD in erp-api-v2/.env, press Enter to use .env)'
    if (-not $env:CRM_DB_PASSWORD) { Remove-Item Env:CRM_DB_PASSWORD }
}
if (-not $env:ERP_JWT_SECRET) {
    $env:ERP_JWT_SECRET = Read-Secret 'JWT_SECRET (same as JWT_SECRET in erp-api-v2/.env, press Enter to use .env)'
    if (-not $env:ERP_JWT_SECRET) { Remove-Item Env:ERP_JWT_SECRET }
}

$logDir = Join-Path (Get-Location) 'tmp'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$log = Join-Path $logDir 'dev-run.log'
Write-Host "Starting backoffice-api on :8090 ... (output also written to $log)"
# Secrets are passed via environment variables only; the app never prints them.
go run ./cmd/api 2>&1 | ForEach-Object { "$_" } | Tee-Object -FilePath $log
Write-Host "backoffice-api exited. Log: $log"
