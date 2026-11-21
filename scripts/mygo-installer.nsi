!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "0.1.0-unsigned"
!endif

!define MUI_ICON "..\prototypes\mygo-run\assets\asterlab.ico"
!define MUI_UNICON "..\prototypes\mygo-run\assets\asterlab.ico"
!define MUI_FINISHPAGE_RUN "$INSTDIR\mygo-run.exe"

Name "AsterLab (${VERSION})"
OutFile "..\release\AsterLab-MyGo-${VERSION}-win-x64-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\AsterLab MyGo"
InstallDirRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "InstallLocation"
RequestExecutionLevel user
SetCompressor /SOLID lzma
SetOverwrite on

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Install"
    SetShellVarContext current
    ; Close any running AsterLab instance in the target folder so in-place upgrades overwrite cleanly.
    IfFileExists "$INSTDIR\mygo-run.exe" 0 doneClose
        DetailPrint "Closing running AsterLab instance for in-place upgrade..."
        System::Call 'Kernel32::SetEnvironmentVariable(t "ASTERLAB_UPGRADE_DIR", t "$INSTDIR")'
        nsExec::ExecToLog 'powershell.exe -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -Command "$$dir = [System.IO.Path]::GetFullPath($$env:ASTERLAB_UPGRADE_DIR).TrimEnd([char]92); Get-Process mygo-run,asterlab -ErrorAction SilentlyContinue | Where-Object { $$_.Path -and [System.IO.Path]::GetDirectoryName($$_.Path).TrimEnd([char]92) -ieq $$dir } | ForEach-Object { $$null = $$_.CloseMainWindow(); if (-not $$_.WaitForExit(2000)) { Stop-Process -Id $$_.Id -Force -ErrorAction SilentlyContinue } }; $$exe = Join-Path $$dir mygo-run.exe; for ($$i=0; $$i -lt 20; $$i++) { try { if (Test-Path -LiteralPath $$exe) { $$fs = [System.IO.File]::Open($$exe, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None); $$fs.Close() }; break } catch { Start-Sleep -Milliseconds 250 } }"'
    doneClose:
    Delete "$INSTDIR\mygo-run.exe.old"
    SetOutPath "$INSTDIR"
    File "..\prototypes\mygo-run\dist\mygo-run.exe"
    File "..\prototypes\mygo-run\dist\asterlab.exe"
    File "..\prototypes\mygo-run\dist\BUILD_INFO.txt"
    File "..\prototypes\mygo-run\assets\asterlab.ico"
    Delete "$INSTDIR\AsterLab-MyGo.bat"

    CreateDirectory "$SMPROGRAMS\AsterLab MyGo"
    CreateShortcut "$SMPROGRAMS\AsterLab MyGo\AsterLab MyGo.lnk" "$INSTDIR\mygo-run.exe" "" "$INSTDIR\asterlab.ico" 0
    CreateShortcut "$DESKTOP\AsterLab MyGo.lnk" "$INSTDIR\mygo-run.exe" "" "$INSTDIR\asterlab.ico" 0

    WriteUninstaller "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "DisplayName" "AsterLab (${VERSION})"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "DisplayIcon" "$INSTDIR\asterlab.ico"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "DisplayVersion" "${VERSION}"
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "NoModify" 1
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "NoRepair" 1
    ; User-level asterlab:// protocol handler (CLOSE-07): per-user, no
    ; elevation. The OS hands the raw URL to the exe as one quoted argument;
    ; strict route parsing happens in-process (internal/state), never shell
    ; execution.
    WriteRegStr HKCU "Software\Classes\asterlab" "" "URL:AsterLab Run Link"
    WriteRegStr HKCU "Software\Classes\asterlab" "URL Protocol" ""
    WriteRegStr HKCU "Software\Classes\asterlab\DefaultIcon" "" "$\"$INSTDIR\asterlab.ico$\",0"
    WriteRegStr HKCU "Software\Classes\asterlab\shell\open\command" "" "$\"$INSTDIR\mygo-run.exe$\" $\"%1$\""
SectionEnd

Section "Uninstall"
    SetShellVarContext current
    Delete "$INSTDIR\mygo-run.exe"
    Delete "$INSTDIR\asterlab.exe"
    Delete "$INSTDIR\BUILD_INFO.txt"
    Delete "$INSTDIR\asterlab.ico"
    Delete "$INSTDIR\AsterLab-MyGo.bat"
    Delete "$INSTDIR\uninstall.exe"
    RMDir "$INSTDIR"
    Delete "$SMPROGRAMS\AsterLab MyGo\AsterLab MyGo.lnk"
    RMDir "$SMPROGRAMS\AsterLab MyGo"
    Delete "$DESKTOP\AsterLab MyGo.lnk"
    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo"
    DeleteRegKey HKCU "Software\Classes\asterlab"
SectionEnd
