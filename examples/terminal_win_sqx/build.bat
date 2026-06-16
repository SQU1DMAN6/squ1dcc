@echo off
REM Build script for terminal_win_sqx
REM Locates sqx_runtime.h automatically relative to the project root.

setlocal enabledelayedexpansion

REM Find the script directory and project root
set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..\..\src\include

REM Find GCC or MSVC
where gcc >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo Building terminal_win.sqx with GCC...
    gcc -O2 -I"%PROJECT_ROOT%" -o "%SCRIPT_DIR%terminal_win.sqx" "%SCRIPT_DIR%main.c"
    if %ERRORLEVEL% EQU 0 (
        echo Successfully built terminal_win.sqx
    ) else (
        echo Build failed with GCC
        exit /b 1
    )
    exit /b 0
)

where cl >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo Building terminal_win.sqx with MSVC...
    cl /O2 /I"%PROJECT_ROOT%" /Fe"%SCRIPT_DIR%terminal_win.sqx" "%SCRIPT_DIR%main.c" /link /out:"%SCRIPT_DIR%terminal_win.sqx"
    if %ERRORLEVEL% EQU 0 (
        echo Successfully built terminal_win.sqx
    ) else (
        echo Build failed with MSVC
        exit /b 1
    )
    exit /b 0
)

echo Error: No supported C compiler found (tried: gcc, cl)
exit /b 1