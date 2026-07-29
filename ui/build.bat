@echo off
setlocal
cd /d "%~dp0"

:: English-only script: cmd.exe default code page breaks UTF-8 Chinese text.
:: Do not add non-ASCII characters to this file.

echo === Fridare GUI Build Tool (Windows) ===
echo.

if not exist build mkdir build

echo Cleaning old build artifacts...
del /q build\fridare-gui.exe 2>nul
del /q build\fridare-create.exe 2>nul
del /q build\fridare-patch.exe 2>nul

echo Building applications...
where fyne >nul 2>nul
if %ERRORLEVEL%==0 (
    echo Using fyne build for GUI...
    fyne build --src cmd/gui -o ../../build/fridare-gui.exe
    if errorlevel 1 goto :error
) else (
    echo fyne CLI not found, falling back to go build...
    echo Tip: go install fyne.io/fyne/v2/cmd/fyne@latest
    go build -ldflags "-H windowsgui" -o build\fridare-gui.exe .\cmd\gui
    if errorlevel 1 goto :error
)

go build -o build\fridare-create.exe .\cmd\create
if errorlevel 1 goto :error
go build -o build\fridare-patch.exe .\cmd\patch
if errorlevel 1 goto :error

echo.
echo Build complete.
echo.
echo Output files:
dir /b build\fridare-*.exe
echo.
echo Run: build\fridare-gui.exe
goto :eof

:error
echo Build failed.
exit /b 1
