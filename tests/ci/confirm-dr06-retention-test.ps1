$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$fixtureRoot = Join-Path $root "tests/.artifacts/dr06-retention-test-$([guid]::NewGuid().ToString('N'))"

try {
    $evidenceRoot = Join-Path $fixtureRoot "evidence"
    $indexPath = Join-Path $fixtureRoot "qualification-index.json"
    $receiptPath = Join-Path $fixtureRoot "receipt.json"
    $artifactPath = Join-Path $evidenceRoot "gate/evidence.txt"
    New-Item -ItemType Directory -Force (Split-Path -Parent $artifactPath) | Out-Null
    Set-Content -Encoding utf8 -LiteralPath $artifactPath -Value "evidence"
    $item = Get-Item -LiteralPath $artifactPath
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $artifactPath).Hash.ToLowerInvariant()
    [ordered]@{
        schema_version = 1
        qualification_accepted = $false
        source = [ordered]@{ commit = "b0b0f951bd06e8ffae47f28669cd350681102839" }
        release_version = "v0.0.0-dr06"
        workflow_run = [ordered]@{ run_id = "fixture"; run_attempt = 1 }
        retention = [ordered]@{ durable_evidence_uri = "urn:ardents:test:retention" }
        artifacts = @([ordered]@{
            path = "gate/evidence.txt"
            size = [long]$item.Length
            sha256 = $hash
        })
    } | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 -LiteralPath $indexPath

    ./tests/ci/confirm-dr06-retention.ps1 `
        -IndexPath $indexPath `
        -EvidenceRoot $evidenceRoot `
        -DurableEvidenceURI "urn:ardents:test:retention" `
        -ReceiptPath $receiptPath
    $receipt = Get-Content -Raw -LiteralPath $receiptPath | ConvertFrom-Json
    if ($receipt.qualification_accepted -ne $false -or
        $receipt.release_owner_review_required -ne $true -or
        $receipt.artifact_count -ne 1) {
        throw "DR-06 durable retention receipt contract mismatch"
    }

    Add-Content -LiteralPath $artifactPath -Value "tampered"
    $rejected = $false
    try {
        ./tests/ci/confirm-dr06-retention.ps1 `
            -IndexPath $indexPath `
            -EvidenceRoot $evidenceRoot `
            -DurableEvidenceURI "urn:ardents:test:retention" `
            -ReceiptPath $receiptPath
    } catch {
        $rejected = $_.Exception.Message -like "*differs from the qualification index*"
    }
    if (-not $rejected) { throw "DR-06 retention verifier accepted tampered evidence" }
    Write-Host "dr06-retention-test=passed"
} finally {
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
