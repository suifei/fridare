@echo off
setlocal enabledelayedexpansion

:: 检查参数数量
if "%~2"=="" (
    echo Usage: %~nx0 ^<frida-server_path^> ^<5_char_magic_name^>
    echo Example: %~nx0 "C:\path\to\frida-server.exe" abcde
    goto :eof
)

:: 获取输入参数
set "FRIDA_SERVER_PATH=%~1"
set "MAGIC_NAME=%~2"

:: 验证输入文件路径
if not exist "%FRIDA_SERVER_PATH%" (
    echo Error: frida-server file not found at %FRIDA_SERVER_PATH%
    goto :eof
)

:: 验证魔改名长度
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

:: 获取脚本所在路径
set "SCRIPT_PATH=%~dp0"
:: 设置 hexreplace 工具路径
set "HEXREPLACE_PATH=%SCRIPT_PATH%hexreplace_windows_amd64.exe"

:: 检查 hexreplace 工具是否存在
if not exist "%HEXREPLACE_PATH%" (
    echo Error: hexreplace tool not found at %HEXREPLACE_PATH%
    goto :eof
)

:: 构建输出文件名
for %%F in ("%FRIDA_SERVER_PATH%") do (
    set "FILE_NAME=%%~nF"
    set "FILE_EXT=%%~xF"
)
set "OUTPUT_PATH=%~dp1%FILE_NAME%%FILE_EXT%_%MAGIC_NAME%"

:: 使用 hexreplace 修改文件
"%HEXREPLACE_PATH%" "%FRIDA_SERVER_PATH%" %MAGIC_NAME% "%OUTPUT_PATH%"

if %errorlevel% neq 0 (
    echo Error occurred during file modification.
    goto :eof
)

echo Modification complete.
echo Modified file saved as: %OUTPUT_PATH%