@echo off
REM Beenet Windows Build Script - Simple version
REM Alternative to Makefile for Windows CMD/PowerShell

echo Building bee...

REM Create build directory if it doesn't exist
if not exist build mkdir build

REM Simple build command with basic version info
go build -o build\bee.exe .\cmd\bee

if %ERRORLEVEL% equ 0 (
    echo Build successful: build\bee.exe
) else (
    echo Build failed with error code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)
