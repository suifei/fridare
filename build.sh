#!/bin/bash

# Frida 魔改脚本，用于修改 frida-server 的名称和端口
# 作者：suifei@gmail.com 

set -e  # 遇到错误立即退出

DEF_FRIDA_VERSION=16.3.3
DEF_FRIDA_SERVER_PORT=8899
DEF_HTTP_PROXY=

YELLOW='\033[33m'
RESET='\033[0m'

yellow_text() {
    local text="$1"
    echo -e "\033[33m${text}\033[0m"
}

# 读取参数
FRIDA_VERSION=${1:-$DEF_FRIDA_VERSION}
FRIDA_SERVER_PORT=${2:-$DEF_FRIDA_SERVER_PORT}
CURL_PROXY=${3:-$DEF_HTTP_PROXY}

echo "Frida 版本：${FRIDA_VERSION}"
echo "Frida 端口：${FRIDA_SERVER_PORT}"

# 函数：检查并安装 dpkg
check_and_install_dpkg() {
    if ! command -v dpkg-deb &> /dev/null; then
        echo "dpkg 未找到，正在使用 Homebrew 安装..."
        brew install dpkg || { echo "安装 dpkg 失败"; exit 1; }
    else
        echo "dpkg 已安装"
    fi
}

# 函数：下载 Frida
download_frida() {
    local arch=$1
    local filename="frida_${FRIDA_VERSION}_iphoneos-${arch}.deb"
    if [ ! -f "$filename" ]; then
        echo "下载 $filename"
        # 如果配置了代理 CURL_PROXY
        if [ -n "$CURL_PROXY" ]; then
            curl -L -o "$filename" --proxy "$CURL_PROXY" "https://github.com/frida/frida/releases/download/${FRIDA_VERSION}/${filename}" || { echo "下载 $filename 失败"; exit 1; }
        else
            curl -L -o "$filename" "https://github.com/frida/frida/releases/download/${FRIDA_VERSION}/${filename}" || { echo "下载 $filename 失败"; exit 1; }
        fi
    else
        echo "本地存在 $filename"
    fi
}

# 新增函数：删除目录内的所有 .DS_Store 文件
remove_ds_store() {
    local dir=$1
    echo "正在删除 $dir 中的 .DS_Store 文件..."
    find "$dir" -name ".DS_Store" -type f -delete
    echo ".DS_Store 文件删除完成"
}

modify_launch_daemon() {
    local build=$1
    local arch=$2
    local plist
    
    if [ "$arch" = "arm64" ]; then
        plist="${build}/var/jb/Library/LaunchDaemons/re.frida.server.plist"
    else
        plist="${build}/Library/LaunchDaemons/re.frida.server.plist"
    fi
    
    if [ ! -f "$plist" ]; then
        echo "错误: plist 文件不存在: $plist"
        return 1
    fi

    echo "正在修改 plist 文件: $plist"
    echo "FRIDA_NAME: $FRIDA_NAME"
    echo "FRIDA_SERVER_PORT: $FRIDA_SERVER_PORT"

    # 检查变量是否为空
    if [ -z "$FRIDA_NAME" ] || [ -z "$FRIDA_SERVER_PORT" ]; then
        echo "错误: FRIDA_NAME 或 FRIDA_SERVER_PORT 为空"
        return 1
    fi

    # 使用 -e 选项为每个替换操作创建单独的 sed 命令
    sed -i '' \
        -e 's/re\.frida\.server/re.'"${FRIDA_NAME}"'.server/g' \
        -e 's/frida-server/'"${FRIDA_NAME}"'/g' \
        -e 's@</array>@<string>-l</string><string>0.0.0.0:'"${FRIDA_SERVER_PORT}"'</string></array>@g' \
        "$plist"

    if [ $? -ne 0 ]; then
        echo "错误: sed 命令执行失败"
        return 1
    fi

    echo "plist 文件修改完成"

    # 重命名 plist 文件
    local new_plist
    if [ "$arch" = "arm64" ]; then
        new_plist="${build}/var/jb/Library/LaunchDaemons/re.${FRIDA_NAME}.server.plist"
    else
        new_plist="${build}/Library/LaunchDaemons/re.${FRIDA_NAME}.server.plist"
    fi
    mv "$plist" "$new_plist"

    if [ $? -ne 0 ]; then
        echo "错误: 重命名 plist 文件失败"
        return 1
    fi

    echo "plist 文件已重命名为: $new_plist"
}

modify_debian_files() {
    local build=$1
    local arch=$2
    local debian_dir
    
    debian_dir="${build}/DEBIAN"
    
    echo "正在修改 DEBIAN 文件夹中的文件: $debian_dir"
    echo "FRIDA_NAME: $FRIDA_NAME"

    # 检查变量是否为空
    if [ -z "$FRIDA_NAME" ]; then
        echo "错误: FRIDA_NAME 为空"
        return 1
    fi

    # 修改 control 文件
    local control_file="${debian_dir}/control"
    if [ -f "$control_file" ]; then
        echo "修改 control 文件"
        sed -i '' 's/re\.frida\.server/re.'"${FRIDA_NAME}"'.server/g' "$control_file"
        if [ $? -ne 0 ]; then
            echo "错误: 修改 control 文件失败"
            return 1
        fi
    else
        echo "警告: control 文件不存在: $control_file"
    fi

    # 修改 extrainst_ 文件
    local extrainst_file="${debian_dir}/extrainst_"
    if [ -f "$extrainst_file" ]; then
        echo "修改 extrainst_ 文件"
        if [ "$arch" = "arm64" ]; then
            sed -i '' 's@launchcfg=/var/jb/Library/LaunchDaemons/re\.frida\.server\.plist@launchcfg=/var/jb/Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$extrainst_file"
        else
            sed -i '' 's@launchcfg=/Library/LaunchDaemons/re\.frida\.server\.plist@launchcfg=/Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$extrainst_file"
        fi
        if [ $? -ne 0 ]; then
            echo "错误: 修改 extrainst_ 文件失败"
            return 1
        fi
    else
        echo "警告: extrainst_ 文件不存在: $extrainst_file"
    fi

    # 修改 prerm 文件
    local prerm_file="${debian_dir}/prerm"
    if [ -f "$prerm_file" ]; then
        echo "修改 prerm 文件"
        if [ "$arch" = "arm64" ]; then
            sed -i '' 's@launchctl unload /var/jb/Library/LaunchDaemons/re\.frida\.server\.plist@launchctl unload /var/jb/Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$prerm_file"
        else
            sed -i '' 's@launchctl unload /Library/LaunchDaemons/re\.frida\.server\.plist@launchctl unload /Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$prerm_file"
        fi
        if [ $? -ne 0 ]; then
            echo "错误: 修改 prerm 文件失败"
            return 1
        fi
    else
        echo "警告: prerm 文件不存在: $prerm_file"
    fi

    echo "DEBIAN 文件夹中的文件修改完成"
}

modify_binary() {
    local build=$1
    local arch=$2
    local frida_server_path
    local new_path
    
    if [ "$arch" = "arm64" ]; then
        frida_server_path="${build}/var/jb/usr/sbin/frida-server"
        new_path="${build}/var/jb/usr/sbin/${FRIDA_NAME}"
    else
        frida_server_path="${build}/usr/sbin/frida-server"
        new_path="${build}/usr/sbin/${FRIDA_NAME}"
    fi
    
    echo "正在修改二进制文件: $frida_server_path"
    
    if [ ! -f "$frida_server_path" ]; then
        echo "错误: frida-server 文件不存在于路径: $frida_server_path"
        return 1
    fi
    
    RES_NAME_HEX=$(echo -n "$FRIDA_NAME" | xxd -pu)
    # 将二进制文件转换为十六进制文本
    xxd -p -c 0 "$frida_server_path" > "${frida_server_path}.hex"
    local replacements=(
        "0066726964612d6d61696e2d6c6f6f70:00${RES_NAME_HEX}2d6d61696e2d6c6f6f70"
        "0066726964615f7365727665725f6170:00${RES_NAME_HEX}5f7365727665725f6170"
        "0066726964615f7365727665725f6d61:00${RES_NAME_HEX}5f7365727665725f6d61"
        "0066726964612d7365727665722d6d61:00${RES_NAME_HEX}2d7365727665722d6d61"
        "00467269646100:00${RES_NAME_HEX}0000"
        "0066726964612d7365727665722d6d61696e2d6c6f:00${RES_NAME_HEX}2d7365727665722d6d61696e2d6c6f"
        "0066726964612d6d61696e2d6c6f:00${RES_NAME_HEX}2d6d61696e2d6c6f"
    )
    
    local success=false

    for replacement in "${replacements[@]}"; do
        IFS=':' read -r search replace <<< "$replacement"
        if sed -i '' "s/$search/$replace/g" "${frida_server_path}.hex"; then
            success=true
            echo "替换完成: $search -> $replace"
        else
            echo "警告: 替换失败 - $search"
        fi
    done
    
    if $success; then
        # 将修改后的十六进制文本转回二进制文件
        xxd -r -p "${frida_server_path}.hex" > "$new_path"
        echo "二进制文件修改完成"
        rm -f "$frida_server_path" "${frida_server_path}.hex"
    else
        echo "警告: 没有成功的替换"
        rm -f "${frida_server_path}.hex"
        return 1
    fi

    # 确保新文件有执行权限
    chmod +x "$new_path"
}

# 函数：重新打包 deb 文件
repackage_deb() {
    local build=$1
    local output_filename=$2
    
    # 在打包之前删除 .DS_Store 文件
    remove_ds_store "$build"

    if [ -f "$output_filename" ]; then
        read -p "${output_filename} 已经存在，是否覆盖？(y/n):" is_cover
        if [ "$is_cover" != "y" ]; then
            output_filename="${output_filename%.*}_1.deb"
        fi
    fi
    
    dpkg-deb -b "$build" "$output_filename" || { echo "打包 $output_filename 失败"; exit 1; }

    rm -rf "$build"
}

# 主函数
main() {
    check_and_install_dpkg
    
    mkdir -p build
    cd build
    
    FRIDA_NAME=$(cat /dev/urandom | env LC_CTYPE=C tr -dc 'a-z' | fold -w 5 | grep -E '^[a-z]+$' | head -n 1)
    
    for arch in arm arm64; do
        download_frida $arch
        
        BUILD_DIR="frida_${FRIDA_VERSION}_iphoneos-${arch}"
        rm -rf "$BUILD_DIR"
        dpkg-deb -R "frida_${FRIDA_VERSION}_iphoneos-${arch}.deb" "$BUILD_DIR"
        
        echo "正在修改 Frida ${FRIDA_VERSION} 版本 (${arch})"
        modify_launch_daemon "$BUILD_DIR" "$arch"
        echo "Launch daemon 修改完成"
        modify_debian_files "$BUILD_DIR" "$arch"
        echo "DEBIAN 文件修改完成"
        modify_binary "$BUILD_DIR" "$arch"
        echo "二进制文件修改完成"
        
        OUTPUT_FILENAME="frida_${FRIDA_VERSION}_iphoneos-${arch}_tcp.deb"
        repackage_deb "$BUILD_DIR" "$OUTPUT_FILENAME"
        
        mkdir -p ../dist
        mv "$OUTPUT_FILENAME" ../dist/
        yellow_text "Frida ${FRIDA_VERSION} 版本 (${arch}) 修改完成"
        yellow_text "新版本名：${FRIDA_NAME}"
        yellow_text "请使用新版本名：${FRIDA_NAME} 进行调试"
        yellow_text "请使用端口：${FRIDA_SERVER_PORT} 进行调试"
        yellow_text "新版本 deb 文件：../dist/${OUTPUT_FILENAME}"
        yellow_text "-------------------------------------------------"
        yellow_text "iPhone 安装："
        yellow_text "scp dist/${OUTPUT_FILENAME} root@<iPhone-IP>:/var/root"
        yellow_text "ssh root@<iPhone-IP>"
        yellow_text "dpkg -i /var/root/${OUTPUT_FILENAME}"
        yellow_text "PC 连接："
        yellow_text "frida -U -f com.xxx.xxx -l"
        yellow_text "frida -H <iPhone-IP>:${FRIDA_SERVER_PORT} -f com.xxx.xxx --no-pause"
        yellow_text "-------------------------------------------------"
    done
    cd ..

}

# 执行主函数
main