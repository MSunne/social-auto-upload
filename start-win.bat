@echo off
TITLE OmniBull Windows Starter

SET SCRIPT_DIR=%~dp0
SET POWERSHELL_EXE=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe
SET ENV_FILE=%SCRIPT_DIR%deploy\env\omnibull.windows.env

IF NOT EXIST "%POWERSHELL_EXE%" (
  ECHO powershell.exe not found.
  EXIT /B 1
)

IF NOT EXIST "%ENV_FILE%" IF EXIST "%SCRIPT_DIR%deploy\env\omnibull.windows.env.example" (
  COPY /Y "%SCRIPT_DIR%deploy\env\omnibull.windows.env.example" "%ENV_FILE%" > nul
)

"%POWERSHELL_EXE%" -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%scripts\sau_windows.ps1" start -EnvFile "%ENV_FILE%"
EXIT /B %ERRORLEVEL%
