@echo off
setlocal
cd /d "%~dp0"
if not defined LABWORKBENCH_PORT set "LABWORKBENCH_PORT=8832"
if not defined LABWORKBENCH_DATA_DIR set "LABWORKBENCH_DATA_DIR=%LOCALAPPDATA%\AsterLab"
start "AsterLab local service" "%~dp0AsterLab.exe"
timeout /t 2 /nobreak >nul
start "" "http://127.0.0.1:%LABWORKBENCH_PORT%"
