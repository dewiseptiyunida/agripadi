param(
    [switch]$LiveLLM,
    [switch]$Strict,
    [string]$Config = ".\config.yml",
    [string]$Output = ".\evaluation-results"
)

$ErrorActionPreference = "Stop"

if (-not $env:APP_ENV) {
    $env:APP_ENV = "evaluation"
}

if (-not $env:DATABASE_DSN) {
    Write-Error "DATABASE_DSN belum diisi. Contoh: `$env:DATABASE_DSN='postgres://postgres:password@127.0.0.1:5432/agripadi?sslmode=disable'"
}

$argsList = @(
    "run", ".\cmd\evaluate",
    "--config", $Config,
    "--rule-cases", ".\dataset\evaluation\rule_cases.json",
    "--llm-cases", ".\dataset\evaluation\llm_cases.json",
    "--output", $Output,
    "--strict=$($Strict.IsPresent.ToString().ToLowerInvariant())"
)

if ($LiveLLM) {
    if (-not $env:LLM_API_KEY -and -not $env:GROQ_API_KEY) {
        Write-Error "LiveLLM dipilih tetapi LLM_API_KEY/GROQ_API_KEY belum diisi."
    }
    $argsList += @("--live-llm", "--llm-delay", "500ms")
}

Write-Host "Menjalankan evaluasi AgriPadi..." -ForegroundColor Cyan
& go @argsList
exit $LASTEXITCODE
