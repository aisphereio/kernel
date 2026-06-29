@echo off
setlocal
set SCRIPT_DIR=%~dp0
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%check-release-modules.ps1" %*
set EXIT_CODE=%ERRORLEVEL%
if not "%EXIT_CODE%"=="0" (
  echo.
  echo release module check failed with exit code %EXIT_CODE%.
)
exit /b %EXIT_CODE%
