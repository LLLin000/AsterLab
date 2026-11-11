@echo off
rem Launch the signed MyGo app directly. Invoking the exe from cmd avoids
rem Explorer's ShellExecute "Open File" prompt; the exe itself is Authenticode
rem signed by scripts/sign-local.ps1.
cd /d "%~dp0"
"%~dp0prototypes\mygo-run\target\asterlab-onevra.exe"
