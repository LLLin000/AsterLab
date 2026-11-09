!include "MUI2.nsh"

Name "AsterLab MyGo (Unsigned Beta)"
OutFile "..\release\AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\AsterLab MyGo"
RequestExecutionLevel user
SilentInstall silent
SilentUnInstall silent
SetCompressor /SOLID lzma

Section "Install"
    SetShellVarContext current
    SetOutPath "$INSTDIR"
    File "..\prototypes\mygo-run\dist\mygo-run.exe"
    File "..\prototypes\mygo-run\dist\Runs.dll"
    File "..\prototypes\mygo-run\dist\BUILD_INFO.txt"
    File "..\scripts\AsterLab-MyGo.bat"

    CreateDirectory "$SMPROGRAMS\AsterLab MyGo"
    CreateShortcut "$SMPROGRAMS\AsterLab MyGo\AsterLab MyGo.lnk" "$INSTDIR\AsterLab-MyGo.bat" "" "$INSTDIR\mygo-run.exe" 0
    CreateShortcut "$DESKTOP\AsterLab MyGo.lnk" "$INSTDIR\AsterLab-MyGo.bat" "" "$INSTDIR\mygo-run.exe" 0

    WriteUninstaller "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "DisplayName" "AsterLab MyGo (Unsigned Beta)"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "DisplayVersion" "0.1.0-beta-unsigned"
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "NoModify" 1
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo" "NoRepair" 1
SectionEnd

Section "Uninstall"
    SetShellVarContext current
    nsExec::ExecToLog 'taskkill /IM mygo-run.exe /F'
    Sleep 500
    Delete "$INSTDIR\mygo-run.exe"
    Delete "$INSTDIR\Runs.dll"
    Delete "$INSTDIR\BUILD_INFO.txt"
    Delete "$INSTDIR\AsterLab-MyGo.bat"
    Delete "$INSTDIR\uninstall.exe"
    RMDir "$INSTDIR"
    Delete "$SMPROGRAMS\AsterLab MyGo\AsterLab MyGo.lnk"
    RMDir "$SMPROGRAMS\AsterLab MyGo"
    Delete "$DESKTOP\AsterLab MyGo.lnk"
    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo"
SectionEnd
