param(
    [Parameter(Mandatory = $true)][string]$FixtureRoot,
    [Parameter(Mandatory = $true)][string]$PreparedOutput,
    [Parameter(Mandatory = $true)][string]$ReaderHost,
    [Parameter(Mandatory = $true)][string]$PublisherHost,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$ArdentsBinary,
    [Parameter(Mandatory = $true)][string]$NodeBinary,
    [Parameter(Mandatory = $true)][string]$ControlBinary,
    [Parameter(Mandatory = $true)][string]$CustodyBinary,
    [Parameter(Mandatory = $true)][string]$SourceCommit,
    [Parameter(Mandatory = $true)][string]$ReaderProvider,
    [Parameter(Mandatory = $true)][ValidateSet('B','MB','MiB','GB','GiB','TB','TiB')][string]$ReaderUnit,
    [Parameter(Mandatory = $true)][UInt64]$ReaderQuantity,
    [Parameter(Mandatory = $true)][string]$PublisherProvider,
    [Parameter(Mandatory = $true)][ValidateSet('B','MB','MiB','GB','GiB','TB','TiB')][string]$PublisherUnit,
    [Parameter(Mandatory = $true)][UInt64]$PublisherQuantity,
    [Parameter(Mandatory = $true)][string]$ReaderPeriodStart,
    [Parameter(Mandatory = $true)][string]$ReaderPeriodEnd,
    [Parameter(Mandatory = $true)][string]$PublisherPeriodStart,
    [Parameter(Mandatory = $true)][string]$PublisherPeriodEnd,
    [Parameter(Mandatory = $true)][UInt64]$ReaderInitialUsedBytes,
    [Parameter(Mandatory = $true)][UInt64]$PublisherInitialUsedBytes,
    [string]$ReaderInterface = 'ens3',
    [string]$PublisherInterface = 'ens1',
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$fixture = [IO.Path]::GetFullPath($FixtureRoot)
$prepared = [IO.Path]::GetFullPath($PreparedOutput)
. (Join-Path $PSScriptRoot 'canonical-json-instant.ps1')

function Resolve-File([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "$Label is absent." }
    return (Resolve-Path -LiteralPath $Path).Path
}
function Assert-Host([string]$Value, [string]$Label) {
    if ($Value -notmatch '^(?:[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?|(?:\d{1,3}\.){3}\d{1,3})$') { throw "$Label is invalid." }
}
function Assert-RemotePath([string]$Value, [string]$Label) {
    if ($Value -notmatch '^/[A-Za-z0-9._/-]+$' -or $Value.Contains('//') -or $Value.Contains('/../')) { throw "$Label is not a canonical remote path." }
}
function Assert-Hex([string]$Value, [string]$Label) {
    if ($Value -cnotmatch '^[0-9a-f]{64}$') { throw "$Label must be lowercase SHA-256-width hex." }
}
function Assert-Name([string]$Value, [string]$Label) {
    if ($Value -cnotmatch '^[A-Za-z0-9._-]{1,128}$') { throw "$Label is invalid." }
}
function Write-Utf8([string]$Path, [string]$Body) {
    [IO.File]::WriteAllText($Path, $Body, [Text.UTF8Encoding]::new($false))
}
function Hash-Text([string]$Value) {
    $bytes = [Text.Encoding]::UTF8.GetBytes($Value)
    return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
}
function Hash-Files([string[]]$Paths) {
    $lines = foreach ($item in $Paths) {
        "$(Split-Path -Leaf $item):$((Get-FileHash -LiteralPath $item -Algorithm SHA256).Hash.ToLowerInvariant())"
    }
    return Hash-Text (($lines | Sort-Object) -join [char]10)
}
function Remote([string]$HostName) { return "$User@$HostName" }
function RemoteTarget([string]$HostName, [string]$Path) { return "$(Remote $HostName):$Path" }
function Invoke-SSH([string]$HostName, [string]$Command, [string]$Label) {
    $lines = @(& ssh @script:sshOptions (Remote $HostName) $Command)
    if ($LASTEXITCODE -ne 0) { throw "$Label failed with exit code $LASTEXITCODE." }
    return $lines
}
function Invoke-Custody([string]$Command, [string[]]$InputLines, [string]$Label) {
    if ($InputLines.Count -eq 0 -or @($InputLines | Where-Object { [string]::IsNullOrEmpty($_) }).Count -ne 0) {
        throw "$Label requires non-empty custody input."
    }
    $wrapped = "set +e; stty -echo; $Command; status=`$?; stty echo; exit `$status"
    ($InputLines -join "`n") | & ssh @script:sshOptions -tt (Remote $PublisherHost) $wrapped
    if ($LASTEXITCODE -ne 0) { throw "$Label failed with exit code $LASTEXITCODE." }
}
function New-CustodySecret() {
    return [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
}
function Save-ProtectedSecret([string]$Path, [string]$Secret) {
    $plain = [Text.Encoding]::UTF8.GetBytes($Secret)
    try {
        $protected = [Security.Cryptography.ProtectedData]::Protect(
            $plain, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
        [IO.File]::WriteAllBytes($Path, $protected)
    } finally {
        [Array]::Clear($plain, 0, $plain.Length)
    }
}
function Send-File([string]$Local, [string]$HostName, [string]$RemotePath, [string]$Label) {
    & scp @script:scpOptions $Local (RemoteTarget $HostName $RemotePath)
    if ($LASTEXITCODE -ne 0) { throw "$Label failed with exit code $LASTEXITCODE." }
}
function Receive-File([string]$HostName, [string]$RemotePath, [string]$Local, [string]$Label) {
    & scp @script:scpOptions (RemoteTarget $HostName $RemotePath) $Local
    if ($LASTEXITCODE -ne 0) { throw "$Label failed with exit code $LASTEXITCODE." }
}
function Host-Address([string]$Role) {
    if ($Role -ceq 'reader') { return $ReaderHost }
    if ($Role -ceq 'publisher') { return $PublisherHost }
    throw "Unknown host role $Role."
}
function Bundle-Path([string]$Relative) {
    return "$($provision.RemoteRoot)/bundle/$($Relative.Replace('\','/'))"
}
function Invoke-Product([string]$HostName, [string]$Binary, [string[]]$Arguments, [string]$Label) {
    $quoted = @($Arguments | ForEach-Object {
        if ($_ -notmatch '^[A-Za-z0-9._:/+,-]+$') { throw "$Label has an unsafe argument." }
        "'$_'"
    })
    return Invoke-SSH $HostName ("'$Binary' " + ($quoted -join ' ')) $Label
}

Assert-Host $ReaderHost 'ReaderHost'
Assert-Host $PublisherHost 'PublisherHost'
Assert-Host $User 'User'
if ($ReaderHost -ceq $PublisherHost) { throw 'Preparation requires two distinct hosts.' }
foreach ($pair in @(@($ReaderInterface, 'ReaderInterface'), @($PublisherInterface, 'PublisherInterface'))) {
    if ($pair[0] -cnotmatch '^[A-Za-z0-9._-]{1,15}$') { throw "$($pair[1]) is invalid." }
}
$key = Resolve-File $SSHKey 'SSHKey'
$ardents = Resolve-File $ArdentsBinary 'ArdentsBinary'
$node = Resolve-File $NodeBinary 'NodeBinary'
$control = Resolve-File $ControlBinary 'ControlBinary'
$custody = Resolve-File $CustodyBinary 'CustodyBinary'
if ($SourceCommit -cnotmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be a full lowercase Git commit.' }
$actualCommit = ((& git -C $repository rev-parse HEAD 2>$null) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or $actualCommit -cne $SourceCommit) { throw 'SourceCommit must equal the checked-out candidate commit.' }
$sourceStatus = @(& git -C $repository status --porcelain --untracked-files=normal)
if ($LASTEXITCODE -ne 0 -or $sourceStatus.Count -ne 0) { throw 'Qualification requires a clean source commit.' }
$inventoryPath = Resolve-File (Join-Path $fixture 'provisioning-inventory.json') 'provisioning inventory'
if ($fixture.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or
    $prepared.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Private fixture and prepared evidence must remain outside the repository.'
}
if (Test-Path -LiteralPath $prepared) { throw 'PreparedOutput must be a new directory.' }
[IO.Directory]::CreateDirectory($prepared) | Out-Null
$inventoryJSON = Get-Content -LiteralPath $inventoryPath -Raw
$provision = $inventoryJSON | ConvertFrom-Json
if ([string]$provision.Schema -cne 'ardents-qualification-provisioning-v1' -or @($provision.State).Count -ne 23 -or
    @($provision.Services).Count -ne 6 -or @($provision.EntryRoots).Count -ne 5) {
    throw 'Provisioning inventory is incomplete.'
}
Assert-Hex ([string]$provision.NetworkID) 'NetworkID'
Assert-Hex ([string]$provision.Seed) 'Seed'
Assert-Hex ([string]$provision.AuthorityPublic) 'State authority'
Assert-RemotePath ([string]$provision.RemoteRoot) 'RemoteRoot'
Assert-RemotePath ([string]$provision.HostingRoot) 'HostingRoot'
$remoteRoot = [string]$provision.RemoteRoot
if ($remoteRoot -cnotmatch '^/var/lib/ardents/qualification/issue60-[0-9a-f]{12}-(tcp-tls|quic)-(net14ad|net14-recovery)-(client-to-publisher|publisher-to-client)$') {
    throw 'RemoteRoot is outside the fixed issue-60 fixture namespace.'
}
$at = Read-CanonicalJSONInstant -JSON $inventoryJSON -Property 'At'
$notAfter = Read-CanonicalJSONInstant -JSON $inventoryJSON -Property 'NotAfter'
$atText = $at.ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
$readerPeriodFrom = [DateTimeOffset]::ParseExact($ReaderPeriodStart, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
$readerPeriodUntil = [DateTimeOffset]::ParseExact($ReaderPeriodEnd, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
$publisherPeriodFrom = [DateTimeOffset]::ParseExact($PublisherPeriodStart, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
$publisherPeriodUntil = [DateTimeOffset]::ParseExact($PublisherPeriodEnd, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
if ($at -ge $notAfter -or $readerPeriodFrom -ge $readerPeriodUntil -or $publisherPeriodFrom -ge $publisherPeriodUntil -or
    $at -lt $readerPeriodFrom -or $notAfter -gt $readerPeriodUntil -or
    $at -lt $publisherPeriodFrom -or $notAfter -gt $publisherPeriodUntil) {
    throw 'Both provider periods must contain the complete fixture validity window.'
}
$now = [DateTimeOffset]::UtcNow
if ($now -lt $at -or ($notAfter - $now) -lt [TimeSpan]::FromHours(4)) {
    throw 'Fixture must be current and retain at least four hours for the complete acceptance matrix.'
}
function Convert-AllowanceToBytes([string]$Unit, [UInt64]$Quantity, [string]$Label) {
    $multipliers = @{ B=[UInt64]1; MB=[UInt64]1000000; MiB=[UInt64](1MB); GB=[UInt64]1000000000;
        GiB=[UInt64](1GB); TB=[UInt64]1000000000000; TiB=[UInt64](1TB) }
    if (-not $multipliers.ContainsKey($Unit) -or $Quantity -eq 0 -or
        $Quantity -gt [UInt64]::MaxValue / [UInt64]$multipliers[$Unit]) {
        throw "$Label provider allowance is invalid."
    }
    return [UInt64]($Quantity * [UInt64]$multipliers[$Unit])
}
$readerLimit = Convert-AllowanceToBytes $ReaderUnit $ReaderQuantity 'Reader'
$publisherLimit = Convert-AllowanceToBytes $PublisherUnit $PublisherQuantity 'Publisher'
if ($ReaderInitialUsedBytes -ge $readerLimit -or $PublisherInitialUsedBytes -ge $publisherLimit) {
    throw 'Initial provider usage must be below the declared allowance.'
}

$script:sshOptions = @('-i', $key, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'StrictHostKeyChecking=accept-new')
$script:scpOptions = @('-i', $key, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'StrictHostKeyChecking=accept-new')
$binaryPaths = @{ ardents="$remoteRoot/commands/ardents"; node="$remoteRoot/commands/ardents-node";
    control="$remoteRoot/commands/ardents-control"; custody="$remoteRoot/commands/ardents-custody" }
foreach ($value in $binaryPaths.Values) { Assert-RemotePath $value 'remote command path' }

foreach ($hostName in @($ReaderHost, $PublisherHost)) {
    [void](Invoke-SSH $hostName "set -eu; test ! -e '$remoteRoot'; install -d -m 755 '$remoteRoot' '$remoteRoot/bundle' '$remoteRoot/commands' '$remoteRoot/handover' '$remoteRoot/clock'" 'create new qualification root')
    & scp @script:scpOptions -r "$fixture\." (RemoteTarget $hostName "$remoteRoot/bundle/")
    if ($LASTEXITCODE -ne 0) { throw "upload private fixture to $hostName failed." }
    Send-File $ardents $hostName $binaryPaths.ardents 'upload ardents'
    Send-File $node $hostName $binaryPaths.node 'upload ardents-node'
    Send-File $control $hostName $binaryPaths.control 'upload ardents-control'
    Send-File $custody $hostName $binaryPaths.custody 'upload ardents-custody'
    [void](Invoke-SSH $hostName "chmod 755 '$remoteRoot/commands/'*; id -u ardents-endpoint >/dev/null 2>&1 || useradd --system --user-group --home-dir /var/lib/ardents/endpoint --shell /usr/sbin/nologin ardents-endpoint" 'install qualification command identities')
    $bundleRoot = "$remoteRoot/bundle"
    $privateRoot = "$bundleRoot/private"
    $nodePrivate = "$privateRoot/nodes"
    $sourcePrivate = "$privateRoot/source"
    $bindPrivate = "set -eu; cd /; test -d '$nodePrivate'; test -d '$sourcePrivate'; test -z `"`$(find '$nodePrivate' '$sourcePrivate' -type l -print -quit)`"; chown root:ardents-endpoint '$bundleRoot' '$privateRoot'; chmod 710 '$bundleRoot' '$privateRoot'; find '$privateRoot' -maxdepth 1 -type f -exec chown root:root '{}' +; find '$privateRoot' -maxdepth 1 -type f -exec chmod 600 '{}' +; chown -R ardents-endpoint:ardents-endpoint '$nodePrivate' '$sourcePrivate'; find '$nodePrivate' '$sourcePrivate' -type d -exec chmod 700 '{}' +; find '$nodePrivate' '$sourcePrivate' -type f -exec chmod 600 '{}' +; test -z `"`$(runuser -u ardents-endpoint -- find '$nodePrivate' '$sourcePrivate' -type f ! -readable -print -quit)`"; runuser -u ardents-endpoint -- test -r '$nodePrivate/00-key.pem'; runuser -u ardents-endpoint -- test -r '$sourcePrivate/0-cert.pem'; runuser -u ardents-endpoint -- test -r '$sourcePrivate/client-key.pem'; ! runuser -u ardents-endpoint -- test -r '$privateRoot/state-authority.pem'"
    [void](Invoke-SSH $hostName $bindPrivate 'bind qualification runtime credentials')
}

$environment = Hash-Files @($ardents, $node, $control, $custody)
$networkCommitment = [string]$provision.NetworkID
$admissionVault = "$remoteRoot/custody/admission"
$serviceVault = "$remoteRoot/custody/service"
$admissionRootCommitment = Hash-Text $admissionVault
$serviceRootCommitment = Hash-Text $serviceVault
foreach ($vault in @($admissionVault, $serviceVault)) {
    [void](Invoke-SSH $PublisherHost "install -d -m 700 '$vault'" 'create custody vault')
}
$admissionSecret = New-CustodySecret
$serviceSecret = New-CustodySecret
Save-ProtectedSecret (Join-Path $prepared 'admission-secret.dpapi') $admissionSecret

function New-Authority([string]$Kind, [string]$Vault, [string]$RootCommitment, [string]$Secret, [string]$ReceiptLabel) {
    $receiptRemote = "$remoteRoot/handover/$ReceiptLabel-authority.json"
    $receiptLocal = Join-Path $prepared "$ReceiptLabel-authority.json"
    $command = "'$($binaryPaths.custody)' create-$Kind-authority --vault-root '$Vault' --environment-commitment '$environment' --network-commitment '$networkCommitment' --root-commitment '$RootCommitment' > '$receiptRemote'"
    Invoke-Custody $command @($Secret, $Secret) "create $Kind authority"
    Receive-File $PublisherHost $receiptRemote $receiptLocal "download $Kind authority receipt"
    $receipt = Get-Content -LiteralPath $receiptLocal -Raw | ConvertFrom-Json
    Assert-Name ([string]$receipt.record_id) "$Kind authority record"
    Assert-Hex ([string]$receipt.id_commitment) "$Kind authority identity"
    Assert-Hex ([string]$receipt.authority_public) "$Kind authority public key"
    return $receipt
}
$admissionAuthority = New-Authority 'admission' $admissionVault $admissionRootCommitment $admissionSecret 'admission'

function Write-HostingPlan([string]$Role, [string]$Provider, [string]$Unit, [string]$Interface, [string]$Start, [string]$End, [UInt64]$Quantity, [UInt64]$InitialUsed, [UInt64]$LowWatermark) {
    $planPath = Join-Path $prepared "hosting-$Role.json"
    $plan = [ordered]@{ schema='ardents-hosting-initialization-v1'; root=[string]$provision.HostingRoot;
        policy=[ordered]@{ provider=$Provider; start=$Start; end=$End; unit=$Unit; quantity=$Quantity;
            directions='tx'; interfaces=@($Interface); initial_used_bytes=$InitialUsed; low_watermark_bytes=$LowWatermark } }
    Write-Utf8 $planPath (($plan | ConvertTo-Json -Depth 8 -Compress) + [Environment]::NewLine)
    $hostName = Host-Address $Role
    $remotePlan = "$remoteRoot/handover/hosting-$Role.json"
    Send-File $planPath $hostName $remotePlan "upload $Role hosting plan"
    $root = [string]$plan.root
    Assert-RemotePath $root "$Role hosting root"
    $state = ((Invoke-SSH $hostName "if test ! -e '$root'; then printf absent; elif test -d '$root' && test -f '$root/period.pin'; then printf ready; else printf invalid; fi" "inspect $Role hosting root") -join '').Trim()
    if ($state -ceq 'absent') {
        [void](Invoke-Product $hostName $binaryPaths.node @('hosting','initialize','--config',$remotePlan) "initialize $Role hosting period")
    } elseif ($state -cne 'ready') {
        throw "$Role hosting root is incomplete."
    }
    $expectedPin = Hash-Text (($plan.policy | ConvertTo-Json -Depth 8 -Compress))
    $actualPin = ((Invoke-SSH $hostName "cat '$root/period.pin'" "verify $Role hosting policy") -join '').Trim()
    if ($actualPin -cne $expectedPin) { throw "$Role hosting policy differs from the retained owner period." }
}
Write-HostingPlan 'reader' $ReaderProvider $ReaderUnit $ReaderInterface $ReaderPeriodStart $ReaderPeriodEnd $ReaderQuantity $ReaderInitialUsedBytes (64GB)
Write-HostingPlan 'publisher' $PublisherProvider $PublisherUnit $PublisherInterface $PublisherPeriodStart $PublisherPeriodEnd $PublisherQuantity $PublisherInitialUsedBytes (128GB)

foreach ($state in @($provision.State)) {
    Assert-Name ([string]$state.Owner) 'State owner'
    Assert-RemotePath ([string]$state.Root) 'State root'
    Assert-RemotePath ([string]$state.Materialization) 'State materialization'
    $hostName = Host-Address ([string]$state.Host)
    $arguments = @('accept-offline','--state-root',[string]$state.Root,'--network-id',[string]$provision.NetworkID,
        '--authorities',[string]$provision.AuthorityPublic,'--threshold','1','--at',$atText,
        '--epoch',[string]$provision.Epoch,'--inputs',[string]$provision.Inputs,'--materialization',[string]$state.Materialization,
        '--profile','ardents-route-v3','--closed-profile-authority',[string]$provision.AuthorityPublic)
    [void](Invoke-Product $hostName $binaryPaths.ardents $arguments "accept State for $($state.Owner)")
}

$issuerPlan = Bundle-Path ([string]$provision.IssuerInitialization)
$issuerPublicRemote = "$remoteRoot/handover/issuer-public.json"
$issuerCommand = "'$($binaryPaths.node)' issuer initialize --config '$issuerPlan' > '$issuerPublicRemote'"
[void](Invoke-SSH $ReaderHost $issuerCommand 'initialize closed issuer')
$issuer = $provision.IssuerNode
$inspectLines = Invoke-Product $ReaderHost $binaryPaths.control @('inspect-closed-issuer-profile','--profile',$issuerPublicRemote,
    '--network',[string]$provision.NetworkID,'--node',[string]$issuer.ID,'--node-key',[string]$issuer.PublicKey) 'inspect closed issuer'
$issuerInspectionJSON = $inspectLines -join [Environment]::NewLine
$issuerInspection = $issuerInspectionJSON | ConvertFrom-Json
$issuerFrom = Read-CanonicalJSONInstant -JSON $issuerInspectionJSON -Property 'NotBefore'
$issuerUntil = Read-CanonicalJSONInstant -JSON $issuerInspectionJSON -Property 'NotAfter'
$expectedIssuerKeys = foreach ($offset in 0..5) {
    $window = $at.AddHours($offset).ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
    foreach ($class in 1..3) { "$window|$class" }
}
$actualIssuerKeys = foreach ($keyRecord in @($issuerInspection.TokenKeys)) {
    $window = ([DateTimeOffset]$keyRecord.WindowStart).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
    if ([string]::IsNullOrWhiteSpace([string]$keyRecord.SPKI)) { throw 'Closed issuer inspection returned an empty public key.' }
    "$window|$([int]$keyRecord.Class)"
}
if ([string]$issuerInspection.Schema -cne 'ardents-closed-issuer-inspection-v1' -or
    $issuerFrom -ne $at -or $issuerUntil -ne $notAfter -or
    ($actualIssuerKeys -join ',') -cne ($expectedIssuerKeys -join ',')) {
    throw 'Closed issuer inspection did not return the exact six-window, three-class key inventory.'
}
Write-Utf8 (Join-Path $prepared 'issuer-inspection.json') (($issuerInspection | ConvertTo-Json -Depth 12 -Compress) + [Environment]::NewLine)

$profileTemplatePath = Resolve-File (Join-Path $fixture ([string]$provision.ClosedProfileTemplate)) 'closed profile template'
$profilePlan = Get-Content -LiteralPath $profileTemplatePath -Raw | ConvertFrom-Json
$profilePlan.IssuanceAuthorityKey = [string]$admissionAuthority.authority_public
$profilePlan.TokenKeys = @($issuerInspection.TokenKeys)
$profilePlanPath = Join-Path $prepared 'closed-profile.json'
Write-Utf8 $profilePlanPath (($profilePlan | ConvertTo-Json -Depth 20 -Compress) + [Environment]::NewLine)
$profilePlanRemote = "$remoteRoot/handover/closed-profile.json"
$signedProfileRemote = "$remoteRoot/handover/closed.profile"
Send-File $profilePlanPath $PublisherHost $profilePlanRemote 'upload closed profile plan'
[void](Invoke-Product $PublisherHost $binaryPaths.control @('sign-closed-profile','--plan',$profilePlanRemote,
    '--authority-key',[string]$provision.AuthorityKey,'--output',$signedProfileRemote) 'sign closed profile')
[void](Invoke-Product $PublisherHost $binaryPaths.control @('inspect-closed-profile','--plan',$profilePlanRemote,
    '--profile',$signedProfileRemote,'--authority',[string]$provision.AuthorityPublic,'--at',$atText) 'inspect closed profile')
$signedProfileLocal = Join-Path $prepared 'closed.profile'
Receive-File $PublisherHost $signedProfileRemote $signedProfileLocal 'download signed closed profile'
Send-File $signedProfileLocal $ReaderHost $signedProfileRemote 'upload signed closed profile to reader'

foreach ($state in @($provision.State)) {
    $hostName = Host-Address ([string]$state.Host)
    [void](Invoke-Product $hostName $binaryPaths.ardents @('accept-closed-profile','--state-root',[string]$state.Root,
        '--network-id',[string]$provision.NetworkID,'--authorities',[string]$provision.AuthorityPublic,'--threshold','1',
        '--at',$atText,'--profile','ardents-route-v3','--closed-profile-authority',[string]$provision.AuthorityPublic,
        '--closed-profile',$signedProfileRemote) "accept closed profile for $($state.Owner)")
}

foreach ($role in @('reader','publisher')) {
    $hostName = Host-Address $role
    $serviceRoots = @($provision.Services | Where-Object { [string]$_.Host -ceq $role } | ForEach-Object {
        Assert-RemotePath ([string]$_.Root) 'Service root'
        "'$([string]$_.Root)'"
    })
    [void](Invoke-SSH $hostName ("install -d -m 755 '$remoteRoot/service'; install -d -o root -g root -m 700 " +
        ($serviceRoots -join ' ')) "prepare $role Service roots")
}
foreach ($service in @($provision.Services)) {
    Assert-Name ([string]$service.Owner) 'Service owner'
    $hostName = Host-Address ([string]$service.Host)
    [void](Invoke-Product $hostName $binaryPaths.ardents @('service-instance','initialize','--config',[string]$service.Plan) "initialize Service Instance $($service.Owner)")
    $serviceAuthority = New-Authority 'service' $serviceVault $serviceRootCommitment $serviceSecret "$($service.Owner)-service"
    $requestLocal = Join-Path $prepared "$($service.Owner)-service.request"
    Receive-File $hostName ([string]$service.Request) $requestLocal "download $($service.Owner) Service request"
    $requestDigest = (Get-FileHash -LiteralPath $requestLocal -Algorithm SHA256).Hash.ToLowerInvariant()
    $requestAuthority = "$remoteRoot/handover/$($service.Owner)-authority.request"
    $responseAuthority = "$remoteRoot/handover/$($service.Owner)-authority.response"
    Send-File $requestLocal $PublisherHost $requestAuthority "upload $($service.Owner) Service request to custody"
    $receiptRemote = "$remoteRoot/handover/$($service.Owner)-service-receipt.json"
    $command = "'$($binaryPaths.custody)' issue-service-credential --vault-root '$serviceVault' --record '$($serviceAuthority.record_id)' --request '$requestAuthority' --response '$responseAuthority' --environment-commitment '$environment' --network-commitment '$networkCommitment' --root-commitment '$serviceRootCommitment' --kind service --id-commitment '$($serviceAuthority.id_commitment)' > '$receiptRemote'"
    Invoke-Custody $command @($requestDigest, $serviceSecret) "issue $($service.Owner) Service credential"
    $credentialReceiptLocal = Join-Path $prepared "$($service.Owner)-service-receipt.json"
    Receive-File $PublisherHost $receiptRemote $credentialReceiptLocal "download $($service.Owner) Service receipt"
    $credentialReceipt = Get-Content -LiteralPath $credentialReceiptLocal -Raw | ConvertFrom-Json
    Assert-Name ([string]$credentialReceipt.record_id) 'Service credential successor'
    if ([string]$credentialReceipt.schema -cne 'ardents-service-credential-response-v1' -or
        [UInt64]$credentialReceipt.generation -ne 1) {
        throw "$($service.Owner) Service credential did not create its first authority generation."
    }
    $responseLocal = Join-Path $prepared "$($service.Owner)-service.response"
    Receive-File $PublisherHost $responseAuthority $responseLocal "download $($service.Owner) Service response"
    $responseOwner = "$remoteRoot/handover/$($service.Owner)-service.response"
    if ($hostName -cne $PublisherHost) { Send-File $responseLocal $hostName $responseOwner "upload $($service.Owner) Service response" }
    else { $responseOwner = $responseAuthority }
    [void](Invoke-Product $hostName $binaryPaths.ardents @('service-instance','accept','--root',[string]$service.Root,
        '--response',$responseOwner) "accept $($service.Owner) Service credential")
}

$authorityInventory = [ordered]@{ Host=$PublisherHost; Binary=$binaryPaths.custody; VaultRoot=$admissionVault;
    RecordID=[string]$admissionAuthority.record_id; EnvironmentCommitment=$environment; NetworkCommitment=$networkCommitment;
    RootCommitment=$admissionRootCommitment; IDCommitment=[string]$admissionAuthority.id_commitment }
$authorityInventoryPath = Join-Path $prepared 'admission-authority-inventory.json'
Write-Utf8 $authorityInventoryPath (($authorityInventory | ConvertTo-Json -Depth 6 -Compress) + [Environment]::NewLine)

foreach ($role in @('reader','publisher')) {
    $hostName = Host-Address $role
    $participantNames = @($provision.Services | Where-Object { [string]$_.Host -ceq $role } | ForEach-Object { [string]$_.Owner })
    $stateRoots = @($provision.State | Where-Object { [string]$_.Host -ceq $role } | ForEach-Object { [string]$_.Root })
    foreach ($stateRoot in $stateRoots) { Assert-RemotePath $stateRoot 'State owner root' }
    [void](Invoke-SSH $hostName "install -d -m 755 '$remoteRoot/state' '$remoteRoot/state-roles' '$remoteRoot/endpoint' '$remoteRoot/service'; chmod 755 '$remoteRoot/bundle' '$remoteRoot/bundle/entry-templates'" "prepare $role Endpoint parents")
    $paths = @($stateRoots)
    $paths += @([string]$provision.HostingRoot, "$remoteRoot/handover", "$remoteRoot/clock")
    foreach ($name in $participantNames) {
        $paths += @("$remoteRoot/state-roles/$name", "$remoteRoot/endpoint/$name",
            "$remoteRoot/service/$name", "$remoteRoot/bundle/entry-templates/$name")
    }
    $quoted = $paths | ForEach-Object { Assert-RemotePath $_ 'Endpoint owner path'; "'$_'" }
    [void](Invoke-SSH $hostName ("install -d -o ardents-endpoint -g ardents-endpoint -m 700 " + ($quoted -join ' ') +
        "; chown -R ardents-endpoint:ardents-endpoint " + ($quoted -join ' ') + "; chmod 755 '$remoteRoot'") "assign $role Endpoint roots")
}

$runInputs = [ordered]@{ Schema='ardents-qualification-prepared-inputs-v1'; ReaderHost=$ReaderHost; PublisherHost=$PublisherHost;
    Seed=[string]$provision.Seed; At=$atText; HostingRoot=[string]$provision.HostingRoot;
    ReaderHostingPolicySHA256=(Get-FileHash -LiteralPath (Join-Path $prepared 'hosting-reader.json') -Algorithm SHA256).Hash.ToLowerInvariant();
    PublisherHostingPolicySHA256=(Get-FileHash -LiteralPath (Join-Path $prepared 'hosting-publisher.json') -Algorithm SHA256).Hash.ToLowerInvariant();
    FixtureRoot=$fixture; ReaderPlanTemplate=(Join-Path $fixture ([string]$provision.ReaderPlanTemplate));
    PublisherPlan=(Join-Path $fixture ([string]$provision.PublisherPlan)); Net32Plan=(Join-Path $fixture ([string]$provision.Net32Plan));
    NodeInventory=(Join-Path $fixture ([string]$provision.NodeInventory));
    NetworkManifest=(Join-Path $fixture 'network-manifest.json'); AuthorityInventory=$authorityInventoryPath;
    RemoteRoot=$remoteRoot; SourceCommit=$SourceCommit; EnvironmentCommitment=$environment; PreparedAt=[DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ') }
Write-Utf8 (Join-Path $prepared 'run-inputs.json') (($runInputs | ConvertTo-Json -Depth 8 -Compress) + [Environment]::NewLine)
Write-Output (Get-Content -LiteralPath (Join-Path $prepared 'run-inputs.json') -Raw)
