param(
    [Parameter(Position = 0)]
    [ValidateSet("help", "up", "status", "start", "stop", "down", "backup", "restore", "upgrade", "rollback", "test", "package")]
    [string]$Command = "help",
    [Parameter(Position = 1)]
    [string]$Target = "",
    [string]$StateDir = "var/deployment/local-multinode",
    [string]$Project = "ardents-local",
    [string]$Image = "ardents/node-local:dev",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 90,
    [switch]$Build,
    [switch]$RemoveVolumes,
    [ValidateSet("seed", "peer2", "peer3")]
    [string]$Node,
    [string]$Archive,
    [switch]$ConfirmReplace,
    [string]$NewImage,
    [switch]$AllowMutableImage,
    [string]$Domain,
    [string]$Scenario,
    [string]$Tag,
    [string]$Profile,
    [string]$ReportDir,
    [string]$JsonReport,
    [string]$JUnitReport,
    [string]$CoverageProfile,
    [string]$ContainerImage = "ardents/test-runner:dev",
    [switch]$RebuildContainer,
    [string]$Commit,
    [string]$OutputDir,
    [long]$SourceDateEpoch = 0
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
Set-Location $root

function Get-DeploymentArguments {
    $values = @{
        StateDir = $StateDir
        Project = $Project
        TimeoutSeconds = $TimeoutSeconds
    }
    return $values
}

switch ($Command) {
    "help" {
        @"
Ardents repository interface

  ./ardents.ps1 up [-Build]       Start and prove a working local network
  ./ardents.ps1 status            Show node and network truth
  ./ardents.ps1 start|stop|down   Manage the local deployment
  ./ardents.ps1 backup|restore    Protect or restore one stopped node
  ./ardents.ps1 upgrade|rollback  Perform a gated rolling rollout
  ./ardents.ps1 test [suite]      Run canonical tests in Docker/Linux
  ./ardents.ps1 package <version> Build release artifacts
"@ | Write-Host
    }
    { $_ -in @("up", "status", "start", "stop", "down") } {
        $params = Get-DeploymentArguments
        $params.Image = $Image
        if ($Build) { $params.Build = $true }
        if ($Command -eq "down" -and $RemoveVolumes) { $params.RemoveVolumes = $true }
        & "$root/scripts/deploy/cluster.ps1" -Action $Command @params
    }
    { $_ -in @("backup", "restore") } {
        if ([string]::IsNullOrWhiteSpace($Node)) { throw "-$Command requires -Node seed|peer2|peer3" }
        $params = Get-DeploymentArguments
        $params.Node = $Node
        if ($Archive) { $params.Archive = $Archive }
        if ($ConfirmReplace) { $params.ConfirmReplace = $true }
        & "$root/scripts/deploy/data.ps1" -Action $Command @params
    }
    { $_ -in @("upgrade", "rollback") } {
        $params = Get-DeploymentArguments
        if ($NewImage) { $params.NewImage = $NewImage }
        if ($AllowMutableImage) { $params.AllowMutableImage = $true }
        & "$root/scripts/deploy/rollout.ps1" -Action $Command @params
    }
    "test" {
        $suite = if ($Target) { $Target } else { "fast" }
        $params = @{
            Suite = $suite
            Domain = $Domain
            Scenario = $Scenario
            Tag = $Tag
            Profile = $Profile
            ReportDir = $ReportDir
            JsonReport = $JsonReport
            JUnitReport = $JUnitReport
            CoverageProfile = $CoverageProfile
            ContainerImage = $ContainerImage
        }
        if ($RebuildContainer) { $params.RebuildContainer = $true }
        & "$root/tests/run.ps1" @params
    }
    "package" {
        if ([string]::IsNullOrWhiteSpace($Target)) { throw "package version is required" }
        $params = @{ Version = $Target; SourceDateEpoch = $SourceDateEpoch }
        if ($Commit) { $params.Commit = $Commit }
        if ($OutputDir) { $params.OutputDir = $OutputDir }
        & "$root/scripts/release/build.ps1" @params
    }
}
