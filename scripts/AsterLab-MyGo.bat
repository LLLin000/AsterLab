@echo off
set "LABWORKBENCH_DATA_DIR=%LOCALAPPDATA%\AsterLab\data"
if not exist "%LABWORKBENCH_DATA_DIR%" mkdir "%LABWORKBENCH_DATA_DIR%"
set "ASTERLAB_RUN_ID=1c0c8989-0513-483a-851b-bdc7d6c67483"
start "" "%~dp0mygo-run.exe"