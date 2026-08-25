[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $ManifestPath,

    [Parameter(Mandatory = $true)]
    [string] $ArtifactPath
)

$ErrorActionPreference = 'Stop'

function Get-PHPModules {
    param([string[]] $Output)

    return @(
        $Output |
            Where-Object { $_ -and -not $_.StartsWith('[') } |
            Sort-Object -Unique
    )
}

$manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json
$runtimes = @($manifest.runtimes)
if ($manifest.schema -ne 1 -or $runtimes.Count -ne 1) {
    throw 'Runtime Manifest must contain exactly one schema-1 runtime'
}

$runtime = $runtimes[0]
$identity = $runtime.identity
foreach ($field in @('id', 'frankenphp_version', 'php_version', 'caddy_version', 'os', 'arch', 'support')) {
    if ([string]::IsNullOrWhiteSpace($identity.$field)) {
        throw "Runtime Identity is missing $field"
    }
}
if ($identity.os -ne 'windows' -or $identity.arch -ne 'x64') {
    throw "Expected a Windows x64 Runtime Identity, got $($identity.os) $($identity.arch)"
}

$actualSHA = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArtifactPath).Hash.ToLowerInvariant()
if ($actualSHA -ne $runtime.artifact.sha256) {
    throw "Runtime artifact checksum mismatch: expected $($runtime.artifact.sha256), got $actualSHA"
}

$extractDirectory = Join-Path ([IO.Path]::GetTempPath()) ('phite-runtime-smoke-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $extractDirectory | Out-Null
Expand-Archive -LiteralPath $ArtifactPath -DestinationPath $extractDirectory

$frankenphpExecutable = Join-Path $extractDirectory 'frankenphp.exe'
$phpExecutable = Join-Path $extractDirectory 'php.exe'
foreach ($requiredFile in @($frankenphpExecutable, $phpExecutable)) {
    if (-not (Test-Path -LiteralPath $requiredFile -PathType Leaf)) {
        throw "Runtime artifact is missing $requiredFile"
    }
}

$frankenphpOutput = @(& $frankenphpExecutable version)
if ($LASTEXITCODE -ne 0) {
    throw "FrankenPHP version command exited with $LASTEXITCODE"
}
$expectedFrankenPHP = "FrankenPHP $($identity.frankenphp_version) PHP $($identity.php_version) Caddy v$($identity.caddy_version) "
if ($frankenphpOutput.Count -ne 1 -or -not $frankenphpOutput[0].StartsWith($expectedFrankenPHP)) {
    throw "Unexpected FrankenPHP identity: $($frankenphpOutput -join [Environment]::NewLine)"
}

$phpVersionOutput = @(& $phpExecutable -n -v)
if ($LASTEXITCODE -ne 0) {
    throw "PHP version command exited with $LASTEXITCODE"
}
$expectedPHP = "PHP $($identity.php_version) "
if ($phpVersionOutput.Count -eq 0 -or -not $phpVersionOutput[0].StartsWith($expectedPHP)) {
    throw "Unexpected PHP identity: $($phpVersionOutput -join [Environment]::NewLine)"
}

$baseModules = Get-PHPModules -Output @(& $phpExecutable -n -m)
if ($LASTEXITCODE -ne 0) {
    throw "PHP module discovery exited with $LASTEXITCODE"
}
$expectedModules = @($identity.extensions | ForEach-Object { $_.ToString() } | Sort-Object -Unique)
if ($expectedModules.Count -eq 0) {
    throw 'Runtime Identity must declare at least one extension'
}

$extensionDirectory = Join-Path $extractDirectory 'ext'
$phpArguments = @('-n', '-d', "extension_dir=$extensionDirectory")
foreach ($extension in $expectedModules) {
    if ($extension -in $baseModules) {
        continue
    }

    $libraryName = 'php_' + $extension.ToLowerInvariant().Replace(' ', '_') + '.dll'
    $libraryPath = Join-Path $extensionDirectory $libraryName
    if (-not (Test-Path -LiteralPath $libraryPath -PathType Leaf)) {
        throw "Runtime Identity declares $extension but $libraryName is absent"
    }
    $phpArguments += @('-d', "extension=$libraryName")
}
$phpArguments += '-m'

$actualModules = Get-PHPModules -Output @(& $phpExecutable @phpArguments)
if ($LASTEXITCODE -ne 0) {
    throw "PHP extension loading exited with $LASTEXITCODE"
}
$moduleDifference = @(Compare-Object -ReferenceObject $expectedModules -DifferenceObject $actualModules)
if ($moduleDifference.Count -ne 0) {
    throw "Runtime extension set differs from the manifest:`n$($moduleDifference | Out-String)"
}

Write-Output "verified $($identity.id)"
