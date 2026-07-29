@echo off
setlocal enabledelayedexpansion

:: English-only script: cmd.exe default code page breaks UTF-8 Chinese text.
:: Do not add non-ASCII characters to this file.

:: Resolve script directory and hexreplace tool
set "SCRIPT_PATH=%~dp0"
set "HEXREPLACE_PATH=%SCRIPT_PATH%hexreplace_windows_amd64.exe"

if not exist "%HEXREPLACE_PATH%" (
    echo Error: hexreplace tool not found at %HEXREPLACE_PATH%
    echo Build it with: cd hexreplace ^&^& go build -o ..\win\hexreplace_windows_amd64.exe
    goto :eof
)

:: Locate Frida package via pip
set "FRIDA_PATH="
for /f "tokens=2 delims= " %%a in ('pip show frida 2^>nul ^| findstr /B "Location"') do set "FRIDA_PATH=%%a\frida"

if "%FRIDA_PATH%"=="" (
    echo Error: cannot locate frida via pip. Install with: pip install frida frida-tools
    goto :eof
)

echo Frida installation path: %FRIDA_PATH%

if not exist "%FRIDA_PATH%\core.py" (
    echo Error: core.py not found in %FRIDA_PATH%
    goto :eof
)

:: Find native extension: _frida.pyd / _frida.abi3.pyd / _frida*.pyd
set "NATIVE_LIB="
if exist "%FRIDA_PATH%\_frida.pyd" set "NATIVE_LIB=%FRIDA_PATH%\_frida.pyd"
if "%NATIVE_LIB%"=="" if exist "%FRIDA_PATH%\_frida.abi3.pyd" set "NATIVE_LIB=%FRIDA_PATH%\_frida.abi3.pyd"
if "%NATIVE_LIB%"=="" (
    for %%f in ("%FRIDA_PATH%\_frida*.pyd") do (
        set "NATIVE_LIB=%%f"
        goto :found_native
    )
)
:found_native

:: Some installs place the extension in the parent site-packages directory
if "%NATIVE_LIB%"=="" (
    for %%f in ("%FRIDA_PATH%\..\_frida*.pyd") do (
        set "NATIVE_LIB=%%f"
        goto :found_native2
    )
)
:found_native2

if "%NATIVE_LIB%"=="" (
    echo Error: _frida*.pyd not found
    echo Searched: %FRIDA_PATH% and its parent directory
    dir /b "%FRIDA_PATH%\_frida*" 2>nul
    goto :eof
)

echo Native lib: %NATIVE_LIB%

:: Backup originals
if not exist "%FRIDA_PATH%\core.py.fridare" (
    copy "%FRIDA_PATH%\core.py" "%FRIDA_PATH%\core.py.fridare" >nul
    echo Backed up core.py
)
if not exist "%NATIVE_LIB%.fridare" (
    copy "%NATIVE_LIB%" "%NATIVE_LIB%.fridare" >nul
    echo Backed up native lib
) else (
    :: Restore clean backup before re-patch to avoid double-patching
    copy /y "%NATIVE_LIB%.fridare" "%NATIVE_LIB%" >nul
)

set /p "input=Please enter 5 lowercase letters (a-z): "
if not "%input:~4,1%" == "" (
    if "%input:~5,1%" == "" (
        echo Input accepted.
    ) else (
        echo Input must be exactly 5 characters.
        goto :eof
    )
) else (
    echo Input must be exactly 5 characters.
    goto :eof
)

echo %input%| findstr /R /C:"^[a-z][a-z][a-z][a-z][a-z]$" >nul
if errorlevel 1 (
    echo Error: magic name must be exactly 5 lowercase letters a-z
    goto :eof
)

"%HEXREPLACE_PATH%" "%NATIVE_LIB%" %input% "%NATIVE_LIB%.modify"
if %errorlevel% neq 0 (
    echo Error occurred during file modification.
    goto :eof
)
move /y "%NATIVE_LIB%.modify" "%NATIVE_LIB%" >nul

:: Patch core.py frida:rpc (idempotent: restore from backup first)
if exist "%FRIDA_PATH%\core.py.fridare" (
    copy /y "%FRIDA_PATH%\core.py.fridare" "%FRIDA_PATH%\core.py" >nul
)
powershell -NoProfile -Command ^
  "$p='%FRIDA_PATH%\core.py'; $c=Get-Content -Raw $p; $n=$c -replace 'frida:rpc','%input%:rpc'; if($c -ne $n){ Set-Content -Path $p -Value $n -NoNewline; Write-Host 'core.py: replaced frida:rpc' } else { Write-Host 'core.py: no frida:rpc found' }"

echo.
echo Modification complete.
echo Magic name: %input%
echo Use the same magic name on the device frida-server.
echo Restore: copy *.fridare files back over the originals.
