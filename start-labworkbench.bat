@echo off
setlocal
cd /d "%~dp0"
if not defined LABWORKBENCH_PORT set "LABWORKBENCH_PORT=8832"
if not defined LABWORKBENCH_DATA_DIR set "LABWORKBENCH_DATA_DIR=%~dp0data"
cargo build --release
if errorlevel 1 exit /b %errorlevel%
set "LABWORKBENCH_EXE=%~dp0target\release\labworkbench.exe"
if not exist "%LABWORKBENCH_EXE%" (echo LabWorkbench build not found.& exit /b 1)
"%LABWORKBENCH_EXE%"