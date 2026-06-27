@echo off
setlocal

REM Windows runner for installing local Kernel tools into .bin.
REM It avoids PowerShell script-signing errors by setting ExecutionPolicy only
REM for this process. It does not change the machine/user policy.

set SCRIPT_DIR=%~dp0
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%tools.ps1" %*
set EXIT_CODE=%ERRORLEVEL%
if not "%EXIT_CODE%"=="0" (
  echo.
  echo tools installation failed with exit code %EXIT_CODE%.
)
exit /b %EXIT_CODE%
