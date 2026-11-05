@echo off
setlocal
cd /d "%~dp0"
if exist "%~dp0target\release\labworkbench.exe" set LABWORKBENCH_CLI=%~dp0target\release\labworkbench.exe
"AsterLab-Slint.exe" --db "docs\prototypes\fixtures\p2-l929-run-demo.sqlite" --renderer software
