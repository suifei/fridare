@echo off
setlocal enabledelayedexpansion

:: 获取脚本所在路径
set "SCRIPT_PATH=%~dp0"
:: 设置 hexreplace 工具路径
set "HEXREPLACE_PATH=%SCRIPT_PATH%hexreplace_windows_amd64.exe"

:: 检查 hexreplace 工具是否存在
if not exist "%HEXREPLACE_PATH%" (
    echo Error: hexreplace tool not found at %HEXREPLACE_PATH%
    goto :eof
)

:: 使用 pip 获取 Frida 安装路径
for /f "tokens=2 delims= " %%a in ('pip show frida ^| findstr "Location"') do set FRIDA_PATH=%%a\frida

echo Frida installation path: %FRIDA_PATH%

:: 检查文件是否存在
if not exist "%FRIDA_PATH%\core.py" (
    echo Error: core.py not found in %FRIDA_PATH%
    goto :eof
)
if not exist "%FRIDA_PATH%\_frida.pyd" (
    echo Error: _frida.pyd not found in %FRIDA_PATH%
    goto :eof
)

:: 备份文件
if not exist "%FRIDA_PATH%\core.py.fridare" (
    copy "%FRIDA_PATH%\core.py" "%FRIDA_PATH%\core.py.fridare"
    echo Backed up core.py
)
if not exist "%FRIDA_PATH%\_frida.pyd.fridare" (
    copy "%FRIDA_PATH%\_frida.pyd" "%FRIDA_PATH%\_frida.pyd.fridare"
    echo Backed up _frida.pyd
)

:: 获取用户输入
set /p "input=Please enter 5 characters (a-z): "
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

:: 使用 hexreplace 修改文件
"%HEXREPLACE_PATH%" "%FRIDA_PATH%\_frida.pyd" %input% "%FRIDA_PATH%\_frida.pyd.modify"

if %errorlevel% neq 0 (
    echo Error occurred during file modification.
    goto :eof
)

:: 替换原文件
move /y "%FRIDA_PATH%\_frida.pyd.modify" "%FRIDA_PATH%\_frida.pyd"

echo Modification complete.