@echo off
setlocal enabledelayedexpansion

:: 获取脚本所在路径
set "SCRIPT_PATH=%~dp0"
:: 设置 hexreplace 工具路径
set "HEXREPLACE_PATH=%SCRIPT_PATH%hexreplace_windows_amd64.exe"

:: 检查 hexreplace 工具是否存在
if not exist "%HEXREPLACE_PATH%" (
    echo Error: hexreplace tool not found at %HEXREPLACE_PATH%
    echo 请先编译: cd hexreplace ^&^& go build -o ..\win\hexreplace_windows_amd64.exe
    goto :eof
)

:: 使用 pip 获取 Frida 安装路径
set "FRIDA_PATH="
for /f "tokens=2 delims= " %%a in ('pip show frida 2^>nul ^| findstr /B "Location"') do set "FRIDA_PATH=%%a\frida"

if "%FRIDA_PATH%"=="" (
    echo Error: 无法通过 pip 定位 frida，请确认已安装: pip install frida frida-tools
    goto :eof
)

echo Frida installation path: %FRIDA_PATH%

if not exist "%FRIDA_PATH%\core.py" (
    echo Error: core.py not found in %FRIDA_PATH%
    goto :eof
)

:: 查找原生库：优先 _frida.pyd / _frida.abi3.pyd / _frida*.pyd
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

:: 部分环境扩展在上层 site-packages
if "%NATIVE_LIB%"=="" (
    for %%f in ("%FRIDA_PATH%\..\_frida*.pyd") do (
        set "NATIVE_LIB=%%f"
        goto :found_native2
    )
)
:found_native2

if "%NATIVE_LIB%"=="" (
    echo Error: 未找到 _frida*.pyd
    echo 已搜索: %FRIDA_PATH% 及其上层目录
    dir /b "%FRIDA_PATH%\_frida*" 2>nul
    goto :eof
)

echo Native lib: %NATIVE_LIB%

:: 备份
if not exist "%FRIDA_PATH%\core.py.fridare" (
    copy "%FRIDA_PATH%\core.py" "%FRIDA_PATH%\core.py.fridare" >nul
    echo Backed up core.py
)
if not exist "%NATIVE_LIB%.fridare" (
    copy "%NATIVE_LIB%" "%NATIVE_LIB%.fridare" >nul
    echo Backed up native lib
) else (
    :: 从干净备份再魔改，避免重复 patch
    copy /y "%NATIVE_LIB%.fridare" "%NATIVE_LIB%" >nul
)

:: 获取用户输入
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

:: 校验小写字母
echo %input%| findstr /R /C:"^[a-z][a-z][a-z][a-z][a-z]$" >nul
if errorlevel 1 (
    echo Error: 魔改名必须是恰好 5 个小写字母 a-z
    goto :eof
)

:: 使用 hexreplace 修改原生库
"%HEXREPLACE_PATH%" "%NATIVE_LIB%" %input% "%NATIVE_LIB%.modify"
if %errorlevel% neq 0 (
    echo Error occurred during file modification.
    goto :eof
)
move /y "%NATIVE_LIB%.modify" "%NATIVE_LIB%" >nul

:: 修改 core.py 中的 frida:rpc（幂等：先还原再替换）
if exist "%FRIDA_PATH%\core.py.fridare" (
    copy /y "%FRIDA_PATH%\core.py.fridare" "%FRIDA_PATH%\core.py" >nul
)
powershell -NoProfile -Command ^
  "$p='%FRIDA_PATH%\core.py'; $c=Get-Content -Raw $p; $n=$c -replace 'frida:rpc','%input%:rpc'; if($c -ne $n){ Set-Content -Path $p -Value $n -NoNewline; Write-Host 'core.py: replaced frida:rpc' } else { Write-Host 'core.py: no frida:rpc found' }"

echo.
echo Modification complete.
echo 魔改名: %input%
echo 请确保设备端 frida-server 使用相同魔改名。
echo 恢复: 将 *.fridare 复制回原文件名即可。
