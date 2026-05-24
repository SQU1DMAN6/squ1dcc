@echo off
setlocal

echo ========================================
echo Building SQU1DCC for Windows x64
echo ========================================

REM Move to script directory
cd /d %~dp0

REM Create output directory
if not exist output mkdir output

echo.
echo Building executable...

cd ..\src

go build -o ..\windows-build\output\squ1dcc.exe .

if %errorlevel% neq 0 (
    echo.
    echo Go build failed.
    exit /b 1
)

echo.
echo Executable built successfully.

cd ..\windows-build

echo.
echo Building MSI installer...

wix build installer.wxs -o SQU1DCC-Installer-x64.msi

if %errorlevel% neq 0 (
    echo.
    echo WiX build failed.
    exit /b 1
)

echo.
echo DONE
