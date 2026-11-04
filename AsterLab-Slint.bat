@echo off
setlocal
cd /d "%~dp0"
"AsterLab-Slint.exe" --db "docs\prototypes\fixtures\l929-demo.sqlite" --renderer software
