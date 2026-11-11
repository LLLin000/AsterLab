param(
    [string]$Path = "prototypes/mygo-run/target/asterlab-onevra.exe",
    [string]$CertSubject = "CN=AsterLab Local Dev"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$signingDir = Join-Path $repoRoot "target\signing"
if (-not (Test-Path $signingDir)) { New-Item -ItemType Directory -Path $signingDir -Force | Out-Null }
$cerPath = Join-Path $signingDir "AsterLabLocalDev.cer"

# Reuse the local code-signing certificate; create one only when absent.
$cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
    Where-Object Subject -eq $CertSubject | Select-Object -First 1
if (-not $cert) {
    $cert = New-SelfSignedCertificate -Subject $CertSubject -Type CodeSigningCert `
        -KeyUsage DigitalSignature -KeyLength 2048 `
        -CertStoreLocation "Cert:\CurrentUser\My" `
        -NotAfter (Get-Date).AddYears(5) `
        -FriendlyName "AsterLab Local Dev Code Signing"
    $cert | Export-Certificate -FilePath $cerPath -Force | Out-Null
    Write-Host "Created cert: $($cert.Thumbprint)"
} else {
    Write-Host "Reusing cert: $($cert.Thumbprint)"
    if (-not (Test-Path $cerPath)) {
        $cert | Export-Certificate -FilePath $cerPath -Force | Out-Null
    }
}

# Trust is local to this user. Import only when missing so reruns stay clean.
foreach ($store in @("Root", "TrustedPublisher")) {
    $existing = Get-ChildItem "Cert:\CurrentUser\$store" |
        Where-Object Thumbprint -eq $cert.Thumbprint
    if (-not $existing) {
        Import-Certificate -FilePath $cerPath -CertStoreLocation "Cert:\CurrentUser\$store" | Out-Null
        Write-Host "Trusted in CurrentUser\$store"
    }
}

if (-not (Test-Path $Path)) { throw "File not found: $Path" }

# Sign with the certificate from the user store. No PFX password is needed.
$sig = Set-AuthenticodeSignature -FilePath $Path -Certificate $cert -HashAlgorithm SHA256
if ($sig.Status -ne "Valid") {
    throw "Signing failed: $($sig.Status) - $($sig.StatusMessage)"
}

Unblock-File -Path $Path -ErrorAction SilentlyContinue

$sig = Get-AuthenticodeSignature -FilePath $Path
Write-Host "Status: $($sig.Status)"
Write-Host "Signer: $($sig.SignerCertificate.Subject)"
Write-Host "Thumbprint: $($sig.SignerCertificate.Thumbprint)"
Write-Host "Path: $((Resolve-Path $Path).Path)"
