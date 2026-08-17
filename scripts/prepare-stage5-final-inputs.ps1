param(
    [Parameter(Mandatory = $true)][string]$ConfigurationRoot,
    [Parameter(Mandatory = $true)][string]$InviteRoot,
    [Parameter(Mandatory = $true)][string]$RouteCredentialRoot
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$target = [IO.Path]::GetFullPath($ConfigurationRoot)
$separator = [IO.Path]::DirectorySeparatorChar
if ($target -eq $repository -or $target.StartsWith($repository + $separator, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Stage 5 final configuration must remain outside the repository'
}
if (Test-Path -LiteralPath $target) {
    throw 'Stage 5 final configuration root must not already exist'
}
$inviteSource = (Resolve-Path -LiteralPath $InviteRoot).Path
$credentialSource = (Resolve-Path -LiteralPath $RouteCredentialRoot).Path
$template = Join-Path $repository 'tests/live/stage5-final/configuration'
$configuration = Join-Path $target 'configuration'
New-Item -ItemType Directory -Path $configuration | Out-Null

foreach ($name in @('topology.json', 'cgroups.json', 'network.json', 'workloads.json', 'observers.json')) {
    Copy-Item -LiteralPath (Join-Path $template $name) -Destination (Join-Path $configuration $name)
}

function Write-CommitmentInventory {
    param([string]$SourceRoot, [string]$Destination)
    $resolved = (Resolve-Path -LiteralPath $SourceRoot).Path
    [string[]]$relativePaths = @(Get-ChildItem -LiteralPath $resolved -File -Recurse | ForEach-Object {
        [IO.Path]::GetRelativePath($resolved, $_.FullName).Replace('\', '/')
    })
    [Array]::Sort($relativePaths, [StringComparer]::Ordinal)
    $lines = $relativePaths | ForEach-Object {
        $relative = $_
        if ($relative -match '[\s]') {
            throw "Secret inventory path contains whitespace: $relative"
        }
        $source = Join-Path $resolved $relative
        $item = Get-Item -LiteralPath $source
        $digest = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant()
        "$digest $($item.Length) $relative"
    }
    if ($lines.Count -eq 0) {
        throw "Secret inventory is empty: $SourceRoot"
    }
    [IO.File]::WriteAllLines($Destination, $lines, [Text.UTF8Encoding]::new($false))
}

Write-CommitmentInventory -SourceRoot $inviteSource -Destination (Join-Path $configuration 'invites.sha256')
Write-CommitmentInventory -SourceRoot $credentialSource -Destination (Join-Path $configuration 'route-credentials.sha256')

$currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
$entries = @((Get-Item -LiteralPath $target)) + @(Get-ChildItem -LiteralPath $target -Force -Recurse)
foreach ($entry in ($entries | Sort-Object { $_.FullName.Length } -Descending)) {
    if (-not $entry.PSIsContainer) {
        $entry.IsReadOnly = $true
    }
    $security = [Security.AccessControl.FileSecurity]::new()
    if ($entry.PSIsContainer) {
        $security = [Security.AccessControl.DirectorySecurity]::new()
        $inheritance = [Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit'
    } else {
        $inheritance = [Security.AccessControl.InheritanceFlags]::None
    }
    $security.SetAccessRuleProtection($true, $false)
    $rule = [Security.AccessControl.FileSystemAccessRule]::new(
        $currentIdentity, [Security.AccessControl.FileSystemRights]'ReadAndExecute', $inheritance,
        [Security.AccessControl.PropagationFlags]::None, [Security.AccessControl.AccessControlType]::Allow)
    $security.AddAccessRule($rule)
    Set-Acl -LiteralPath $entry.FullName -AclObject $security
}

Write-Output $target
