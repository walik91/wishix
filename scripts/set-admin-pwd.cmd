@echo off
setlocal
if not exist "%ProgramFiles%\Git\bin\bash.exe" (
  echo Git Bash was not found at "%ProgramFiles%\Git\bin\bash.exe". 1>&2
  exit /b 1
)
"%ProgramFiles%\Git\bin\bash.exe" "%~dp0set-admin-pwd.sh" %*
exit /b %ERRORLEVEL%
