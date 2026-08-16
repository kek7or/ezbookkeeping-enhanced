@echo off
rem Double-click this to bring up Ollama, the backend and the frontend, then
rem open the app. Pin it to the taskbar or put a shortcut on the desktop.
rem
rem Everything it does is "make up" - this file exists so that starting the app
rem does not require opening a terminal and remembering which directory it is in.
rem
rem The url is asked of make rather than written here, so the port lives in
rem exactly one place: WEB_PORT at the top of the Makefile.

cd /d "%~dp0"

title ezBookkeeping - starting

echo Starting ezBookkeeping...
echo.

make up
if errorlevel 1 goto failed

for /f "delims=" %%u in ('make --no-print-directory url') do set "APP_URL=%%u"
if defined APP_URL start "" "%APP_URL%"
exit /b 0

:failed
echo.
echo Something did not start. The window stays open so the error above can be read.
echo Logs are in log\ - server.err, web.err, ollama.err
echo.
pause
exit /b 1
