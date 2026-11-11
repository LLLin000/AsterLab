# Signing and release boundary

## Local development signing (current)

- Certificate: self-signed `CN=AsterLab Local Dev`.
- Trust: `Cert:\CurrentUser\Root` and `Cert:\CurrentUser\TrustedPublisher`.
- `Get-AuthenticodeSignature` reports `Valid` for the built app.
- Script: `scripts/sign-local.ps1`; build+sign: `scripts/build-mygo.ps1`.
- Rebuilding the exe invalidates the signature; re-sign after every build.

This is a **development identity**, not public trust. It does not make the app
trusted on another machine.

## Why Windows can still ask before opening

The observed dialog is the Attachment Manager prompt:

> 你想运行此文件吗？ / Do you want to run this file?

It is separate from publisher verification. The publisher is already shown as
`AsterLab Local Dev`, but Windows still treats `.exe` as a medium-risk
attachment and asks before launching. The policy that suppresses it
(`HKCU\...\Policies\Associations\LowRiskFileTypes`) is not writable without
administrator or Group Policy.

For local development, `AsterLab.cmd` and the desktop `AsterLab` shortcut invoke
the exe directly and bypass the ShellExecute attachment prompt. They are
convenience launchers, not release artifacts.

## Production release checklist

- Use a publicly trusted code-signing certificate (OV/EV) or a signed MSIX /
  Store package. Self-signed certificates do not satisfy SmartScreen reputation.
- Sign the application exe, installer, and any executable that users launch.
  Timestamp every signature.
- Keep the private key in CI secrets or an HSM; never ship the PFX or its
  password with the build.
- Test from a clean Windows VM, with the package downloaded through a browser:
  Mark-of-the-Web, SmartScreen, installer signature, UAC, and first launch.
- Test a second machine that has never seen the development certificate.
- Record the signing identity, thumbprint, timestamp, and verification output
  with the release.
