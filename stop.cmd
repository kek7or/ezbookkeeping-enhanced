@echo off
rem Double-click this to stop Ollama, the backend and the frontend.
rem
rem Stopping Ollama frees whatever VRAM the model was holding, which is the
rem reason to bother stopping rather than leaving it running.

cd /d "%~dp0"

title ezBookkeeping - stopping

make down

exit /b 0
