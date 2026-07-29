@echo off
setlocal enabledelayedexpansion

:: English-only script: cmd.exe default code page breaks UTF-8 Chinese text.
:: Do not add non-ASCII characters to this file.

if "%~2"=="" (
    echo Usage: %~nx0 ^<frida-server_path^> ^<5_char_magic_name^>
    echo Example: %~nx0 "C:\path\to\frida-server.exe" abcde
    goto :eof
)

set "FRIDA_SERVER_PATH=%~1"
set "MAGIC_NAME=%~2"

if not exist "%FRIDA_SERVER_PATH%" (
    echo Error: frida-server file not found at %FRIDA_SERVER_PATH%
    goto :eof
)

if not "%MAGIC_NAME:~4,1%" == "" (
    if "%MAGIC_NAME:~5,1%" == "" (
        echo Magic name accepted.
    ) else (
        echo Error: Magic name must be exactly 5 characters.
        goto :eof
    )
) else (
    echo Error: Magic name must be exactly 5 characters.
    goto :eof
)

echo %MAGIC_NAME%| findstr /R /C:"^[a-z][a-z][a-z][a-z][a-z]$" >nul
if errorlevel 1 (
    echo Error: magic name must be exactly 5 lowercase letters a-z
    goto :eof
)

set "SCRIPT_PATH=%~dp0"
set "HEXREPLACE_PATH=%SCRIPT_PATH%hexreplace_windows_amd64.exe"

if not exist "%HEXREPLACE_PATH%" (
    echo Error: hexreplace tool not found at %HEXREPLACE_PATH%
    echo Build it with: cd hexreplace ^&^& go build -o ..\win\hexreplace_windows_amd64.exe
    goto :eof
)

for %%F in ("%FRIDA_SERVER_PATH%") do (
    set "FILE_NAME=%%~nF"
    set "FILE_EXT=%%~xF"
)
set "OUTPUT_PATH=%~dp1%FILE_NAME%%FILE_EXT%_%MAGIC_NAME%"

"%HEXREPLACE_PATH%" "%FRIDA_SERVER_PATH%" %MAGIC_NAME% "%OUTPUT_PATH%"

if %errorlevel% neq 0 (
    echo Error occurred during file modification.
    goto :eof
)

echo Modification complete.
echo Modified file saved as: %OUTPUT_PATH%
