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
RequestExecutionLevel user
SetCompressor /SOLID lzma

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Install"
    SetShellVarContext current
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
