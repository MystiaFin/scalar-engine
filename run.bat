@echo off
rem run        start scalar
rem run test   run the tests
rem run pull   read older emails

cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo go or ollama not found, run this, reopen the terminal, then try again:
  echo winget install GoLang.Go Ollama.Ollama
  exit /b 1
)

if "%~1"=="" (
  go run ./cmd/main.go
) else if "%~1"=="test" (
  go test ./...
) else if "%~1"=="pull" (
  go run ./cmd/pull
) else (
  echo usage: run [test^|pull]
  exit /b 1
)
