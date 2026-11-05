@echo off
setlocal
cd /d "%~dp0"
"AsterLab-Slint.exe" --db "docs\prototypes\fixtures\p2-l929-run-demo.sqlite" --renderer software
