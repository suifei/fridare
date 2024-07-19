#!/bin/bash

# Frida 魔改脚本，用于修改 frida-server 的名称和端口
# 作者：suifei@gmail.com

set -e # 遇到错误立即退出

VERSION="3.1.5"
# 默认值设置
DEF_FRIDA_SERVER_PORT=8899
DEF_AUTO_CONFIRM="false"

readonly COLOR_YELLOW='\033[33m'
readonly COLOR_RED='\033[31m'
readonly COLOR_PINK='\033[95m'
readonly COLOR_MAGENTA='\033[95m'
readonly COLOR_ORANGE='\033[33m'
readonly COLOR_PURPLE='\033[35m'
readonly COLOR_GREEN='\033[32m'
readonly COLOR_BLUE='\033[34m'
readonly COLOR_SKYBLUE='\033[36m'
readonly COLOR_WHITE='\033[37m'
readonly COLOR_BLACK='\033[30m'
readonly COLOR_GRAY='\033[90m'
readonly COLOR_CYAN='\033[96m'
readonly COLOR_BOLD='\033[1m'
readonly COLOR_DIM='\033[2m'
readonly COLOR_ITALIC='\033[3m'
readonly COLOR_UNDERLINE='\033[4m'
readonly COLOR_BLINK='\033[5m'
readonly COLOR_REVERSE='\033[7m'
readonly COLOR_STRIKETHROUGH='\033[9m'

readonly COLOR_RESET='\033[0m'
readonly COLOR_BG_WHITE='\033[47m'
readonly COLOR_BG_BLACK='\033[40m'
readonly COLOR_BG_RED='\033[41m'
readonly COLOR_BG_GREEN='\033[42m'
readonly COLOR_BG_YELLOW='\033[43m'
readonly COLOR_BG_BLUE='\033[44m'
readonly COLOR_BG_PURPLE='\033[45m'
readonly COLOR_BG_SKYBLUE='\033[46m'

# 初始化变量
FRIDA_VERSION=""
FRIDA_SERVER_PORT=""
CURL_PROXY=""
AUTO_CONFIRM=""
CONFIG_FILE="${HOME}/.fridare.conf"

# 免责声明
DISCLAIMER="${COLOR_BG_WHITE}${COLOR_BLACK}${COLOR_BLINK} 本脚本仅供${COLOR_BLUE}学习使用，${COLOR_RED}请勿用于非法用途。${COLOR_RESET}
免责声明:
1. 本脚本仅供学习和研究使用，不得用于商业或非法目的。
2. 使用本脚本修改Frida可能违反Frida的使用条款或版权规定。
3. 用户应自行承担使用本脚本的所有风险和法律责任。
4. 脚本作者不对因使用本脚本而导致的任何损失或法律问题负责。
5. 使用本脚本即表示您已阅读并同意以上声明。

Disclaimer:
1. This script is for learning and research purposes only and may not be used 
   for commercial or illegal purposes.
2. Using this script to modify Frida may violate Frida's terms of use or 
   copyright regulations.
3. Users should bear all risks and legal responsibilities of using this script.
4. The script author is not responsible for any losses or legal issues caused 
   by the use of this script.
5. Using this script means that you have read and agree to the above statement.
"

# 日志函数
log_info() {
    echo -e "${COLOR_WHITE}[INFO] $1${COLOR_RESET}"
}

log_success() {
    echo -e "${COLOR_GREEN}[SUCC] $1${COLOR_RESET}"
}

log_warning() {
    echo -e "${COLOR_YELLOW}[WARN] $1${COLOR_RESET}"
}

log_error() {
    echo -e "${COLOR_BG_WHITE}${COLOR_RED}[ERRO] $1${COLOR_RESET}"
}

log_cinfo() {
    echo -e "$1[INFO] $2${COLOR_RESET}"
}

log_color() {
    echo -e "$1$2${COLOR_RESET}"
}

log_skyblue() {
    echo -e "${COLOR_SKYBLUE}$1${COLOR_RESET}"
}
download_with_progress() {
    local url="$1"
    local output_file="$2"
    local description="$3"
    local retry_count=3
    local temp_file="${output_file}.tmp"

    local curl_cmd="curl -L --progress-bar --fail --show-error"
    if [ -n "$CURL_PROXY" ]; then
        case "$CURL_PROXY" in
        socks4://*)
            local proxy=${CURL_PROXY#socks4://}
            curl_cmd+=" --socks4 $proxy"
            ;;
        socks4a://*)
            local proxy=${CURL_PROXY#socks4a://}
            curl_cmd+=" --socks4a $proxy"
            ;;
        socks5://*)
            local proxy=${CURL_PROXY#socks5://}
            curl_cmd+=" --socks5 $proxy"
            ;;
        socks5h://*)
            local proxy=${CURL_PROXY#socks5h://}
            curl_cmd+=" --socks5-hostname $proxy"
            ;;
        http://*)
            curl_cmd+=" --proxy $CURL_PROXY"
            ;;
        https://*)
            curl_cmd+=" --proxy $CURL_PROXY"
            ;;
        *)
            log_warning "未知的代理协议，将作为 HTTP 代理使用: $CURL_PROXY"
            curl_cmd+=" --proxy $CURL_PROXY"
            ;;
        esac
    fi

    while [ $retry_count -gt 0 ]; do
        echo -e "${COLOR_SKYBLUE}正在下载 $description...${COLOR_RESET}"

        if $curl_cmd "$url" -o "$temp_file" 2>&1 | tee /dev/stderr |
            sed -u 's/^[# ]*\([0-9]*\.[0-9]%\).*\([ 0-9.]*\(KiB\|MiB\|GiB\)\/s\).*$/\1\n速度: \2/' |
            while IFS= read -r line; do
                if [[ $line =~ ^[0-9]+\.[0-9]% ]]; then
                    percent=${line%\%*}
                    completed=$(printf "%.0f" $percent)
                    printf "\r进度: [%-50s] %d%%" $(printf "=%.0s" $(seq 1 $((completed / 2)))) "$completed"
                elif [[ $line =~ ^速度: ]]; then
                    printf " %s" "$line"
                fi
            done; then
            echo # 换行
            mv "$temp_file" "$output_file"
            local file_size=$(wc -c <"$output_file")
            echo -e "${COLOR_GREEN}下载完成: $output_file (大小: $file_size 字节)${COLOR_RESET}"
            return 0
        else
            echo # 换行
            local curl_exit_code=$?
            log_error "下载失败: $output_file (curl exit code: $curl_exit_code)"
            if [ -f "$temp_file" ]; then
                log_error "临时文件大小: $(wc -c <"$temp_file") 字节"
                rm -f "$temp_file"
            fi
            retry_count=$((retry_count - 1))
            if [ $retry_count -gt 0 ]; then
                log_warning "检查网络连接..."
                if ! ping -c 1 github.com &>/dev/null; then
                    log_error "无法连接到 github.com，请检查网络连接"
                    return 1
                fi
                log_warning "网络连接正常，5秒后重试..."
                sleep 5
            fi
        fi
    done

    if [ $retry_count -eq 0 ]; then
        log_error "达到最大重试次数，下载失败"
        return 1
    fi
}
show_main_usage() {
    echo -e "${COLOR_SKYBLUE}Frida 重打包工具 v${VERSION}${COLOR_RESET}"
    # 计算并显示当前脚本的 MD5 值
    local script_path="$0"
    local script_md5=$(md5 -q "$script_path" 2>/dev/null || md5sum "$script_path" | cut -d ' ' -f 1)
    echo -e "${COLOR_GREEN}脚本 MD5: ${COLOR_YELLOW}${script_md5}${COLOR_RESET}"
    echo -e "${COLOR_WHITE}用法: $0 <命令> [选项]${COLOR_RESET}"
    echo
    echo -e "${COLOR_YELLOW}命令:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}q,    quickstart${COLOR_RESET}     显示快速开始指南"
    echo -e "  ${COLOR_GREEN}b,    build${COLOR_RESET}          重新打包 Frida"
    echo -e "  ${COLOR_GREEN}p,    patch${COLOR_RESET}          修补指定的 Frida 模块"
    echo -e "  ${COLOR_GREEN}pt,   patch-tools${COLOR_RESET}    修补 frida-tools 模块"
    echo -e "  ${COLOR_GREEN}pwt,  patch-wintools${COLOR_RESET} 修补 Windows frida-tools 模块"
    echo -e "  ${COLOR_GREEN}ls,   list${COLOR_RESET}           列出可用的 Frida 版本"
    echo -e "  ${COLOR_GREEN}dl,   download${COLOR_RESET}       下载特定版本的 Frida"
    echo -e "  ${COLOR_GREEN}lm,   list-modules${COLOR_RESET}   列出可用的 Frida 模块"
    echo -e "  ${COLOR_GREEN}s,    setup${COLOR_RESET}          检查并安装系统依赖"
    echo -e "  ${COLOR_GREEN}conf, config${COLOR_RESET}         设置配置选项"
    echo -e "  ${COLOR_GREEN}u,    upgrade${COLOR_RESET}        更新配置，检查新版本"
    echo -e "  ${COLOR_GREEN}h,    help${COLOR_RESET}           显示帮助信息"
    echo
    echo -e "${COLOR_WHITE}运行 '$0 help <命令>' 以获取特定命令的更多信息。${COLOR_RESET}"
    echo -e "${COLOR_WHITE}新用户？ 运行 '$0 quickstart' 获取快速入门指南。${COLOR_RESET}"
    echo -e "${COLOR_GRAY}    suifei@gmail.com${COLOR_RESET}"
    echo -e "${COLOR_GRAY}    https://github.com/suifei/fridare${COLOR_RESET}"
}

show_build_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 b|build [选项]${COLOR_RESET}"
    echo
    echo -e "${COLOR_YELLOW}选项:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}-c clean${COLOR_RESET}                                     清理构建目录"
    echo -e "  ${COLOR_GREEN}-v VERSION${COLOR_RESET}                                   指定 Frida 版本"
    echo -e "  ${COLOR_GREEN}-latest${COLOR_RESET}                                      使用最新的 Frida 版本"
    echo -e "  ${COLOR_GREEN}-p, --port PORT${COLOR_RESET}                              指定 Frida 服务器端口 (默认: $DEF_FRIDA_SERVER_PORT)"
    echo -e "  ${COLOR_GREEN}-y, --yes${COLOR_RESET}                                    自动回答是以确认提示"
    echo -e "  ${COLOR_GREEN}-l, --local archs[arm,arm64,arm64e] FILENAME${COLOR_RESET} 使用本地 deb 文件，指定构建架构"
    echo
    echo -e "${COLOR_BG_WHITE}${COLOR_RED}注意: -v, -latest 和 -l 不能同时使用${COLOR_RESET}"
    echo -e "${COLOR_BG_WHITE}${COLOR_RED}注意: -l 使用本地 deb 文件时，请使用全路径，或者不要把原始包放到 build 目录，会导致路径冲突。${COLOR_RESET}"
    echo -e "${COLOR_WHITE}示例:${COLOR_RESET}"
    echo -e "  $0 build -v 16.4.2"
    echo -e "  $0 build -latest"
    echo -e "  $0 build -l arm64 frida-server_16.4.2_amd64.deb"
    echo -e "  $0 build -c -l arm64 frida-server_16.4.2_amd64.deb "
    echo -e "  $0 build -c -l arm64 frida-server_16.4.2_amd64.deb" -p 8000"
    echo -e " $0 build -c -l arm64 frida-server_16.4.2_amd64.deb" -p 8000 -y"
}

show_patch_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 patch [选项]${COLOR_RESET}"
    echo
    echo -e "${COLOR_YELLOW}选项:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}-m, --module NAME${COLOR_RESET}        指定要修补的 Frida 模块名称"
    echo -e "  ${COLOR_GREEN}-v, --version VERSION${COLOR_RESET}    指定 Frida 版本"
    echo -e "  ${COLOR_GREEN}-latest${COLOR_RESET}                  使用最新的 Frida 版本"
    echo -e "  ${COLOR_GREEN}-os OS${COLOR_RESET}                   指定操作系统 (可选)"
    echo -e "  ${COLOR_GREEN}-arch ARCH${COLOR_RESET}               指定处理器架构 (可选)"
    echo -e "  ${COLOR_GREEN}-o, --output DIR${COLOR_RESET}         指定输出目录 (默认: ./patched_output)"
    echo -e "  ${COLOR_GREEN}-n, --no-backup${COLOR_RESET}          不保留源文件 (默认保留)"
    echo -e "  ${COLOR_GREEN}-a, --auto-package${COLOR_RESET}       自动打包修补后的模块 (默认不打包)"
    echo -e "  ${COLOR_GREEN}-f, --force${COLOR_RESET}              覆盖已存在的文件 (默认跳过)"
    echo
    echo -e "${COLOR_WHITE}示例:${COLOR_RESET}"
    echo -e "  $0 patch -m frida-server -v 14.2.18 -os android -arch arm64 -o ./patched -a"
    echo -e "  $0 patch -m frida-gadget -latest -os ios -arch arm64 -k -a -f"
}
show_patch_tools_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 patch-tools <操作> [选项]${COLOR_RESET}"
    echo
    echo -e "${COLOR_YELLOW}操作:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}name${COLOR_RESET}                     配置 frida-tools 魔改名，5个（a-zA-Z）字符，留空则读取配置的名称\"${COLOR_GREEN}${FRIDA_NAME}${COLOR_RESET}\"，否则随机生成"
    echo -e "  ${COLOR_GREEN}restore${COLOR_RESET}                  恢复 frida-tools 到原版"
    echo
    echo -e "${COLOR_WHITE}示例:${COLOR_RESET}"
    echo -e "  $0 patch-tools name abcde"
    echo -e "  $0 patch-tools restore"
}
show_config_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 config <操作> <选项> [<值>]${COLOR_RESET}"
    echo
    echo -e "${COLOR_YELLOW}操作:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}set <选项> <值>${COLOR_RESET}    设置配置"
    echo -e "  ${COLOR_GREEN}unset <选项>${COLOR_RESET}       取消设置"
    echo -e "  ${COLOR_GREEN}ls, list${COLOR_RESET}          列出所有配置"
    echo -e "  ${COLOR_GREEN}edit${COLOR_RESET}              启动交互式配置编辑器"
    echo -e "  ${COLOR_GREEN}frida-tools${COLOR_RESET}       安装 frida-tools"
    echo
    echo -e "${COLOR_YELLOW}选项:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}proxy${COLOR_RESET}              HTTP 代理"
    echo -e "  ${COLOR_GREEN}port${COLOR_RESET}               Frida 服务器端口"
    echo -e "  ${COLOR_GREEN}frida-name${COLOR_RESET}         Frida 魔改名"
}

show_download_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 dl|download [选项] <输出目录>${COLOR_RESET}"
    echo
    echo -e "${COLOR_YELLOW}选项:${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}-v, --version VERSION${COLOR_RESET}    指定要下载的 Frida 版本"
    echo -e "  ${COLOR_GREEN}-latest${COLOR_RESET}                  下载最新的 Frida 版本"
    echo -e "  ${COLOR_GREEN}-m, --module MODULE${COLOR_RESET}      指定要下载的模块名称"
    echo -e "  ${COLOR_GREEN}-all${COLOR_RESET}                     下载所有模块"
    echo -e "  ${COLOR_GREEN}--no-extract${COLOR_RESET}             不自动解压文件"
    echo -e "  ${COLOR_GREEN}-f, --force${COLOR_RESET}              覆盖已存在的文件 (默认跳过)"
    echo -e "  ${COLOR_GREEN}lm, list-modules${COLOR_RESET}         列出所有可用的模块"
    echo
    echo -e "${COLOR_WHITE}示例:${COLOR_RESET}"
    echo -e "  $0 download -v 16.4.2 -m frida-server ./output"
    echo -e "  $0 download -latest -m frida-gadget ./output"
    echo -e "  $0 download -latest -all ./output -f"
    echo -e "  $0 download -latest -all --no-extract ./output"
}

show_setup_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 s|setup${COLOR_RESET}"
    echo
    echo -e "${COLOR_WHITE}检查并安装系统依赖。这个命令将：${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 检查所有必要的依赖是否已安装"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 如果缺少任何依赖，尝试安装它们"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 创建必要的目录结构（如 build 和 dist 目录）"
    echo
    echo -e "${COLOR_YELLOW}此命令不需要任何额外的参数。${COLOR_RESET}"
}

show_upgrade_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 u|upgrade${COLOR_RESET}"
    echo
    echo -e "${COLOR_WHITE}更新配置并检查新版本。这个命令将：${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 更新 FRIDA_MODULES 配置"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 检查 fridare.sh 脚本的新版本"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 如果有新版本可用，提示用户是否要更新"
    echo
    echo -e "${COLOR_YELLOW}此命令不需要任何额外的参数。${COLOR_RESET}"
}

show_list_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 ls|list${COLOR_RESET}"
    echo
    echo -e "${COLOR_WHITE}列出可用的 Frida 版本。这个命令将：${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 从 GitHub 获取最新的 Frida 发布版本信息"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 显示最近的 10 个版本，包括版本号、发布日期和下载次数"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 为每个版本显示简短的更新说明"
    echo
    echo -e "${COLOR_YELLOW}此命令不需要任何额外的参数。${COLOR_RESET}"
}

show_list_modules_usage() {
    echo -e "${COLOR_SKYBLUE}用法: $0 lm|list-modules${COLOR_RESET}"
    echo
    echo -e "${COLOR_WHITE}列出可用的 Frida 模块。这个命令将：${COLOR_RESET}"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 显示所有可用的 Frida 模块"
    echo -e "  ${COLOR_GREEN}•${COLOR_RESET} 包括模块名称、支持的操作系统和架构"
    echo
    echo -e "${COLOR_YELLOW}此命令不需要任何额外的参数。${COLOR_RESET}"
}

quick_start_guide() {
    echo -e "${COLOR_SKYBLUE}快速开始指南${COLOR_RESET}"
    echo -e "${COLOR_GREEN}1.${COLOR_RESET} 设置环境: $0 setup"
    echo -e "${COLOR_GREEN}2.${COLOR_RESET} 查看可用的 Frida 版本: $0 list"
    echo -e "${COLOR_GREEN}3.${COLOR_RESET} 构建 Frida: $0 build -v <版本> 或 $0 build -latest"
    echo -e "${COLOR_GREEN}4.${COLOR_RESET} 下载 Frida 模块: $0 download -latest -m frida-server ./output"
    echo -e "${COLOR_GREEN}5.${COLOR_RESET} 修补 Frida 模块: $0 patch -m frida-server -latest"
    echo -e "${COLOR_GREEN}6.${COLOR_RESET} 配置设置: $0 config edit"
    echo -e "\n${COLOR_YELLOW}详细使用说明请运行: $0 help <命令>${COLOR_RESET}"
}
# 函数：解析命令行参数
parse_arguments() {
    if [ $# -eq 0 ]; then
        show_main_usage
        exit 0
    fi

    command="$1"
    shift

    case "$command" in
    b | build)
        parse_build_args "$@"
        ;;
    p | patch)
        parse_patch_args "$@"
        ;;
    s | setup)
        setup_environment
        ;;
    conf | config)
        parse_config_args "$@"
        ;;
    ls | list)
        list_frida_versions
        ;;
    dl | download)
        parse_download_args "$@"
        ;;
    lm | list-modules)
        list_frida_modules
        ;;
    u | upgrade)
        update_frida_modules
        check_version "false"
        ;;
    q | quickstart)
        quick_start_guide
        ;;
    h | help)
        if [ $# -eq 0 ]; then
            show_main_usage
        else
            case "$1" in
            b | build) show_build_usage ;;
            p | patch) show_patch_usage ;;
            conf | config) show_config_usage ;;
            dl | download) show_download_usage ;;
            s | setup) show_setup_usage ;;
            u | upgrade) show_upgrade_usage ;;
            ls | list) show_list_usage ;;
            lm | list-modules) show_list_modules_usage ;;
            *)
                log_error "未知命令: $1"
                show_main_usage
                ;;
            esac
        fi
        ;;
    install)
        update_frida_modules
        is_install="true"
        check_version $is_install
        ;;
    pt | patch-tools)
        parse_patch_tools_args "$@"
        ;;
    *)
        log_error "未知命令: $command"
        show_main_usage
        exit 1
        ;;
    esac
}
parse_patch_tools_args() {
    local action=""
    local new_name=""

    while [[ $# -gt 0 ]]; do
        case $1 in
        name)
            action="name"
            if [[ -n "$2" && ! "$2" =~ ^- ]]; then
                new_name="$2"
                shift
            fi
            shift
            ;;
        restore)
            action="restore"
            shift
            ;;
        *)
            log_error "无效的参数: $1"
            show_patch_tools_usage
            exit 1
            ;;
        esac
    done

    if [ -z "$action" ]; then
        log_error "必须指定 name 或 restore 操作"
        show_patch_tools_usage
        exit 1
    fi

    local python_cmd=$(get_python_cmd)
    if [ -z "$python_cmd" ]; then
        log_error "未找到 Python 解释器"
        return 1
    fi

    local pylib_path=$($python_cmd -c "import os, frida; print(os.path.dirname(frida.__file__))" 2>/dev/null)
    if [ $? -ne 0 ]; then
        log_error "执行 Python 命令失败，请确保 frida 已正确安装"
        return 1
    fi

    log_success "找到 frida-tools 路径: $pylib_path"
    log_skyblue "是否确认使用此路径？"
    read -p "请输入 (y/n)" -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "操作已取消"
        exit 0
    fi

    if [ "$action" = "name" ]; then
        patch_frida_tools "$new_name"
    elif [ "$action" = "restore" ]; then
        restore_frida_tools "$pylib_path"
    fi
}
get_latest_frida_version() {
    local latest_version=$(curl -s "https://api.github.com/repos/frida/frida/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$latest_version" ]; then
        log_error "无法获取最新的 Frida 版本"
        exit 1
    fi
    echo "$latest_version"
}
parse_build_args() {
    FRIDA_SERVER_PORT="$DEF_FRIDA_SERVER_PORT"
    AUTO_CONFIRM="false"
    USE_LATEST="false"
    FRIDA_VERSION=""
    clean="false"
    LOCAL_DEB=""
    LOCAL_ARCH=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
        -v)
            if [ "$USE_LATEST" = "true" ] || [ -n "$LOCAL_DEB" ]; then
                log_error "错误: -v 和 -latest 或 -l 不能同时使用" >&2
                show_build_usage
                exit 1
            fi
            FRIDA_VERSION="$2"
            shift 2
            ;;
        -latest)
            if [ -n "$FRIDA_VERSION" ] || [ -n "$LOCAL_DEB" ]; then
                log_error "错误: -latest 和 -v 或 -l 不能同时使用" >&2
                show_build_usage
                exit 1
            fi
            USE_LATEST="true"
            shift
            ;;
        -p | --port)
            FRIDA_SERVER_PORT="$2"
            shift 2
            ;;
        -y | --yes)
            AUTO_CONFIRM="true"
            shift
            ;;
        -c | --clean)
            clean="true"
            shift
            ;;
        -l | --local)
            if [ -n "$FRIDA_VERSION" ] || [ "$USE_LATEST" = "true" ]; then
                log_error "错误: -l 和 -v 或 -latest 不能同时使用" >&2
                show_build_usage
                exit 1
            fi
            if [ -z "$2" ] || [ -z "$3" ]; then
                log_error "错误: 使用本地文件时必须指定处理器架构 arm, arm64 或 arm64e" >&2
                show_build_usage
                exit 1
            fi
            LOCAL_ARCH="$2"
            LOCAL_DEB="$3"
            shift 3
            ;;
        *)
            log_error "无效选项: $1" >&2
            show_build_usage
            exit 1
            ;;
        esac
    done

    if [ "$USE_LATEST" = "true" ]; then
        FRIDA_VERSION=$(get_latest_frida_version)
        log_info "使用最新的 Frida 版本: $FRIDA_VERSION"
    elif [ -z "$FRIDA_VERSION" ] && [ -z "$LOCAL_DEB" ]; then
        log_error "错误: 必须指定 Frida 版本 (-v) 或使用最新版本 (-latest) 或使用本地文件 (-l)" >&2
        show_build_usage
        exit 1
    else
        log_info "使用指定的 Frida 版本: $FRIDA_VERSION"
    fi

    # 执行构建逻辑
    build_frida "$clean" "$LOCAL_DEB" "$LOCAL_ARCH"
}
parse_config_args() {
    if [ $# -eq 0 ]; then
        show_config_usage
        exit 1
    fi

    action="$1"
    shift

    case "$action" in
    set)
        if [ $# -lt 2 ]; then
            log_error "set 命令需要一个选项和一个值"
            show_config_usage
            exit 1
        fi
        option="$1"
        value="$2"
        set_config "$option" "$value"
        list_config
        ;;
    unset)
        if [ $# -lt 1 ]; then
            log_error "unset 命令需要一个选项"
            show_config_usage
            exit 1
        fi
        option="$1"
        unset_config "$option"
        list_config
        ;;
    ls | list)
        list_config
        ;;
    edit)
        interactive_config_editor
        ;;
    frida-tools)
        install_frida_tools
        ;;
    *)
        log_error "未知的配置操作: $action"
        show_config_usage
        exit 1
        ;;
    esac
}

set_config() {
    local option="$1"
    local value="$2"
    case "$option" in
    proxy)
        CURL_PROXY="$value"
        log_success "代理已设置为: $CURL_PROXY"
        ;;
    port)
        FRIDA_SERVER_PORT="$value"
        log_success "Frida 服务器端口已设置为: $FRIDA_SERVER_PORT"
        ;;
    frida-name)
        FRIDA_NAME="$value"
        log_success "Frida 魔改名已设置为: $FRIDA_NAME"
        ;;
    *)
        log_error "未知的配置选项: $option"
        show_config_usage
        return
        ;;
    esac
    update_config_file
}

unset_config() {
    local option="$1"
    case "$option" in
    proxy)
        CURL_PROXY=""
        log_success "HTTP 代理设置已取消"
        ;;
    port)
        FRIDA_SERVER_PORT="$DEF_FRIDA_SERVER_PORT"
        log_success "Frida 服务器端口已重置为默认值: $FRIDA_SERVER_PORT"
        ;;
    frida-name)
        FRIDA_NAME=""
        log_success "Frida 魔改名设置已取消"
        ;;
    *)
        log_error "未知的配置选项: $option"
        show_config_usage
        return
        ;;
    esac
    update_config_file
}
list_config() {
    log_info "当前配置 (存储在 $CONFIG_FILE)："
    [ -n "$CURL_PROXY" ] && log_info "HTTP 代理: $CURL_PROXY"
    log_info "Frida 服务器端口: ${FRIDA_SERVER_PORT:-$DEF_FRIDA_SERVER_PORT}"
    [ -n "$FRIDA_NAME" ] && log_info "Frida 魔改名: $FRIDA_NAME"
}
update_config_file() {
    echo "#Fridare.sh config" >"$CONFIG_FILE"
    echo "FRIDA_SERVER_PORT=${FRIDA_SERVER_PORT}" >>"$CONFIG_FILE"
    echo "CURL_PROXY=${CURL_PROXY}" >>"$CONFIG_FILE"
    echo "AUTO_CONFIRM=${AUTO_CONFIRM}" >>"$CONFIG_FILE"
    echo "FRIDA_NAME=${FRIDA_NAME}" >>"$CONFIG_FILE"
    echo "FRIDA_MODULES=(" >>"$CONFIG_FILE"
    for module in "${FRIDA_MODULES[@]}"; do
        echo "$module" >>"$CONFIG_FILE"
    done
    echo ")" >>"$CONFIG_FILE"
    log_success "配置已更新: $CONFIG_FILE"
}

interactive_config_editor() {
    while true; do
        echo -e "${COLOR_SKYBLUE}交互式配置编辑器${COLOR_RESET}"
        echo -e "${COLOR_GREEN}1.${COLOR_RESET} 编辑 WEB 代理 ${COLOR_GRAY}(当前: ${CURL_PROXY:-未设置})${COLOR_RESET}"
        echo -e "${COLOR_GREEN}2.${COLOR_RESET} 编辑 Frida 服务器端口 ${COLOR_GRAY}(当前: ${FRIDA_SERVER_PORT:-$DEF_FRIDA_SERVER_PORT})${COLOR_RESET}"
        echo -e "${COLOR_GREEN}3.${COLOR_RESET} 编辑 Frida 魔改名 ${COLOR_GRAY}(当前: ${FRIDA_NAME:-未设置})${COLOR_RESET}"
        echo -e "${COLOR_GREEN}4.${COLOR_RESET} 退出"
        read -p "请选择要编辑的项目 (1-4): " choice
        case $choice in
        1)
            read -p "输入新的 HTTP 代理 (当前: ${CURL_PROXY:-未设置}): " new_proxy
            if [ -n "$new_proxy" ]; then
                set_config proxy "$new_proxy"
            else
                echo -e "${COLOR_YELLOW}保持原值不变${COLOR_RESET}"
            fi
            ;;
        2)
            read -p "输入新的 Frida 服务器端口 (当前: ${FRIDA_SERVER_PORT:-$DEF_FRIDA_SERVER_PORT}): " new_port
            if [ -n "$new_port" ]; then
                set_config port "$new_port"
            else
                echo -e "${COLOR_YELLOW}保持原值不变${COLOR_RESET}"
            fi
            ;;
        3)
            read -p "输入新的 Frida 魔改名 (当前: ${FRIDA_NAME:-未设置}): " new_name
            if [ -n "$new_name" ]; then
                # 检查新名称是否有效
                if [[ ! "$new_name" =~ ^[a-zA-Z]{5}$ ]]; then
                    log_error "无效的魔改名: $new_name"
                    log_info "魔改名必须是恰好 5 个字母（a-z 或 A-Z）"
                else
                    set_config frida-name "$new_name"
                fi
            else
                echo -e "${COLOR_YELLOW}保持原值不变${COLOR_RESET}"
            fi
            ;;
        4)
            return
            ;;
        *)
            echo -e "${COLOR_BG_WHITE}${COLOR_RED}无效的选择${COLOR_RESET}"
            ;;
        esac
        echo
    done
}
render_markdown() {
    echo "$1" | sed -E '
        # 标题
        s/^# (.+)$/\n\\033[1;4;31m\1\\033[0m\n/g;
        s/^## (.+)$/\n\\033[1;4;32m\1\\033[0m\n/g;
        s/^### (.+)$/\n\\033[1;4;33m\1\\033[0m\n/g;
        s/^#### (.+)$/\n\\033[1;4;34m\1\\033[0m\n/g;
        s/^##### (.+)$/\n\\033[1;4;35m\1\\033[0m\n/g;
        s/^###### (.+)$/\n\\033[1;4;36m\1\\033[0m\n/g;

        # 粗体和斜体
        s/\*\*\*([^*]+)\*\*\*/\\033[1;3m\1\\033[0m/g;
        s/\*\*([^*]+)\*\*/\\033[1m\1\\033[0m/g;
        s/\*([^*]+)\*/\\033[3m\1\\033[0m/g;
        s/_([^_]+)_/\\033[3m\1\\033[0m/g;

        # 删除线
        s/~~([^~]+)~~/\\033[9m\1\\033[0m/g;

        # 代码块
        s/`([^`]+)`/\\033[7m\1\\033[0m/g;

        # 链接
        s/\[([^\]]+)\]\(([^\)]+)\)/\\033[4;34m\1\\033[0m (\\033[34m\2\\033[0m)/g;

        # 无序列表
        s/^[*+-] (.+)$/  • \1/g;

        # 有序列表 (仅支持前9个项目)
        s/^([1-9])\. (.+)$/  \1. \2/g;

        # 水平线
        s/^([-*_]{3,})$/\\033[37m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\\033[0m/g;

        # 引用
        s/^> (.+)$/  ┃ \\033[36m\1\\033[0m/g;
    '
}
list_frida_versions() {
    log_info "获取 Frida 最新版本列表..."

    # 使用 GitHub API 获取最新的 10 个发布版本
    releases=$(curl -s "https://api.github.com/repos/frida/frida/releases?per_page=10")

    if [ $? -ne 0 ]; then
        log_error "无法从 GitHub 获取 Frida 版本信息"
        exit 1
    fi

    log_success "最新的 Frida 版本："
    log_color ${COLOR_BG_WHITE}${COLOR_BLUE} "序号\t版本\t\t发布日期\t\t下载次数"
    echo -e "$(render_markdown "---")"

    echo "$releases" | jq -r '.[] | "\(.tag_name)\t\(.published_at)\t\(.assets | length)"' |
        while IFS=$'\t' read -r version date asset_count; do
            # 格式化日期 (适用于 macOS)
            formatted_date=$(date -jf "%Y-%m-%dT%H:%M:%SZ" "$date" "+%Y-%m-%d" 2>/dev/null || echo "$date")
            description=$(echo "$releases" | jq -r ".[] | select(.tag_name == \"$version\") | .body" | sed 's/\r//' | sed 's/\n\n/\n/g' | head -n 10)
            # 输出格式化的版本信息
            rendered_description=$(render_markdown "$description")
            printf "${COLOR_GREEN}%2d${COLOR_RESET}\t${COLOR_YELLOW}%-10s${COLOR_RESET}\t%s\t\t%s\n" "$((++i))" "$version" "$formatted_date" "$asset_count"
            echo -e "${rendered_description}"
            echo -e "$(render_markdown "---")"
        done

    echo -e "\n${COLOR_SKYBLUE}提示: 使用 'fridare.sh build -v <版本号>' 来构建特定版本${COLOR_RESET}"
}
FRIDA_MODULES=()
update_frida_modules() {
    log_info "正在更新 FRIDA_MODULES..."
    local python_cmd=$(get_python_cmd)
    if [ -z "$python_cmd" ]; then
        log_error "未找到 Python 解释器"
        return 1
    fi

    # 确保 CURL_PROXY 环境变量可用于 Python 脚本
    export CURL_PROXY
    pip install requests[socks] >/dev/null 2>&1
    local new_modules=$($python_cmd -c "$(
        cat <<EOF
import os
import re
import requests

def get_proxy_settings():
    curl_proxy = os.environ.get('CURL_PROXY', '')
    if curl_proxy:
        if curl_proxy.startswith('socks5://'):
            return {"http": curl_proxy, "https": curl_proxy}
        elif curl_proxy.startswith(('http://', 'https://')):
            return {"http": curl_proxy, "https": curl_proxy}
        else:
            print(f"警告：不支持的代理协议: {curl_proxy}")
            return None
    return None

def get_latest_release():
    url = "https://api.github.com/repos/frida/frida/releases/latest"
    proxies = get_proxy_settings()
    try:
        response = requests.get(url, proxies=proxies)
        response.raise_for_status()
        return response.json()
    except requests.RequestException as e:
        print(f"Error fetching latest release: {str(e)}")
        return None

def parse_filename(filename):
    pattern = re.compile(r'(?P<module_type>frida[-_]?[a-zA-Z0-9\-]+)?[-_](?P<version>v?\d+\.\d+\.\d+)[-_](?P<os>[a-zA-Z0-9\-]+)[-_](?P<arch>[a-zA-Z0-9_]+)')
    match = pattern.search(filename)
    if match:
        module_type = match.group('module_type') or 'N/A'
        version = match.group('version')
        os = match.group('os')
        arch = match.group('arch')
        if '.' in filename:
            ext = filename.split('.')[-1]
        else:
            ext = 'N/A'
        if '64' == arch:
            arch = 'x86_64'
        elif '_' in arch:
            arch = arch.split('_')[-1]
        if os.startswith('cp') and '-' in os:
            os_info = os.split('-')
            os = os_info[-1]
            module_type = 'frida-python-' + os_info[0] + '-' + os_info[1]
        elif 'node-' in os:
            os_info = os.split('-')
            os = os_info[-1]
            module_type = 'frida-' + os_info[0] + '-' + os_info[1]
        elif 'electron-' in os:
            os_info = os.split('-')
            os = os_info[-1]
            module_type = 'frida-' + os_info[0] + '-' + os_info[1]
        elif '-' in os:
            os = os.split('-')[0]

        if 'N/A' == module_type and 'deb' in filename:
            module_type = 'frida-' + os + '-deb'
        elif 'N/A' == module_type and 'gum-graft' in filename:
            module_type = 'gum-graft'
            os = filename.split('-')[-2]
            arch = filename.split('-')[-1].split('.')[0]
        return (module_type, version, os, arch, ext)
    return None

def generate_frida_modules():
    release = get_latest_release()
    if not release:
        return []

    assets = release['assets']
    frida_modules = []

    for asset in assets:
        parsed = parse_filename(asset['name'])
        if parsed:
            module_type, version, os, arch, ext = parsed
            if 'v' == version[0]:
                version = version[1:]
            asset_name = asset['name'].replace(version, '{VERSION}')
            frida_modules.append(f'"{module_type}:{os}:{arch}:{asset_name}"')

    return frida_modules

def main():
    frida_modules = generate_frida_modules()
    for module in frida_modules:
        print(f"    {module}")

if __name__ == "__main__":
    main()
EOF
    )")

    if [ $? -ne 0 ]; then
        log_error "更新 FRIDA_MODULES 失败"
        return 1
    fi

    FRIDA_MODULES=()
    FRIDA_MODULES=("$new_modules")
    update_config_file
    # 更新配置文件中的 FRIDA_MODULES
    # sed -i '' '/^FRIDA_MODULES=/,/^)/d' "$CONFIG_FILE"
    # echo "$new_modules" >>"$CONFIG_FILE"

    log_success "FRIDA_MODULES 更新完成"
    return 0
}
list_frida_modules() {
    # 检查 FRIDA_MODULES 是否为空
    if [ ${#FRIDA_MODULES[@]} -eq 0 ]; then
        log_info "FRIDA_MODULES 为空，正在更新..."
        update_frida_modules
        # 重新加载配置文件以获取更新后的 FRIDA_MODULES
        source "$CONFIG_FILE"
    fi

    log_info "可用的 Frida 模块："
    # 使用临时文件来存储和排序唯一的模块
    temp_file=$(mktemp)

    # 添加表头
    echo -e "模块名称\t操作系统\t架构" >"$temp_file"
    echo -e "-------\t-------\t----" >>"$temp_file"

    for item in "${FRIDA_MODULES[@]}"; do
        IFS=':' read -r mod os arch filename <<<"$item"
        echo -e "$mod\t$os\t$arch" >>"$temp_file"
    done

    # 使用 column 命令对齐输出，并删除临时文件
    column -t -s $'\t' <"$temp_file"
    rm -f "$temp_file"
}

parse_download_args() {
    local version=""
    local module=""
    local output_dir=""
    local use_latest=false
    local download_all=false
    local no_extract=false
    local os=""
    local arch=""
    local force=false

    while [[ $# -gt 0 ]]; do
        case $1 in
        ls | list-modules)
            list_frida_modules
            exit 0
            ;;
        -v | --version)
            version="$2"
            shift 2
            ;;
        -latest)
            use_latest=true
            shift
            ;;
        -m | --module)
            module="$2"
            shift 2
            ;;
        -all)
            download_all=true
            shift
            ;;
        --no-extract)
            no_extract=true
            shift
            ;;
        -os)
            os="$2"
            shift 2
            ;;
        -arch)
            arch="$2"
            shift 2
            ;;
        -f | --force)
            force=true
            shift
            ;;
        *)
            if [[ -z "$output_dir" ]]; then
                output_dir="$1"
                shift
            else
                log_error "无效的参数: $1"
                show_download_usage
                exit 1
            fi
            ;;
        esac
    done

    if [[ -z "$output_dir" ]]; then
        log_error "必须指定输出目录"
        show_download_usage
        exit 1
    fi

    if [[ "$use_latest" == true && -n "$version" ]]; then
        log_error "不能同时指定版本和使用最新版本"
        show_download_usage
        exit 1
    fi

    if [[ "$download_all" == true && -n "$module" ]]; then
        log_error "不能同时指定模块和下载所有模块"
        show_download_usage
        exit 1
    fi

    # 执行下载逻辑
    download_frida_module "$version" "$use_latest" "$module" "$download_all" "$output_dir" "$no_extract" "$os" "$arch" "$force"
}
download_frida_module() {
    local version="$1"
    local use_latest="$2"
    local module="$3"
    local download_all="$4"
    local output_dir="$5"
    local no_extract="$6"
    local os="$7"
    local arch="$8"
    local force="$9"

    # 如果使用最新版本，获取最新版本号
    if [[ "$use_latest" == true ]]; then
        version=$(get_latest_frida_version)
        log_info "使用最新版本: $version"
    fi
    # 如果指定了模块，检查模块是否存在
    if [[ -n "$module" ]]; then
        local module_found=false
        for item in "${FRIDA_MODULES[@]}"; do
            IFS=':' read -r item_mod item_os item_arch filename <<<"$item"
            if [[ "$item_mod" == "$module" ]]; then
                module_found=true
                break
            fi
        done
        if [[ "$module_found" == false ]]; then
            log_error "指定的模块 '$module' 不存在。使用 'download list-modules' 查看可用模块。"
            exit 1
        fi
    fi
    # 检查 7z 是否安装
    if ! command -v 7z &>/dev/null; then
        log_warning "7z 未安装，将使用系统默认解压工具"
    fi

    # 创建基础输出目录
    mkdir -p "$output_dir"

    local found_match=false
    # 遍历模块列表
    for item in "${FRIDA_MODULES[@]}"; do
        IFS=':' read -r item_mod item_os item_arch filename <<<"$item"
        # 如果指定了模块且不匹配，则跳过
        if [ "$item_mod" != "$module" ]; then
            # 如果不是下载全部，找到匹配项后就退出循环
            if [[ "$download_all" != true ]]; then
                continue
            fi
        fi
        found_match=true

        # 替换文件名中的版本占位符
        filename="${filename/\{VERSION\}/$version}"

        # 创建目录结构
        local dir="${output_dir}/${version}/${item_mod}/${item_os}/${item_arch}"
        mkdir -p "$dir"

        local url="https://github.com/frida/frida/releases/download/${version}/${filename}"
        local output_file="${dir}/${filename}"

        if [[ -f "$output_file" && "$force" != true ]]; then
            log_info "文件 $filename 已存在，跳过下载"
        else
            log_info "正在下载 $filename 到 $dir"
            download_with_progress "$url" "$output_file" "$filename"

            log_success "下载 $filename 完成"
        fi
        # 解压逻辑
        if [[ "$no_extract" != true ]]; then
            if [[ "$filename" != *.deb && "$filename" != *.whl ]]; then # 排除 deb 和 whl 文件
                if command -v 7z &>/dev/null; then
                    log_info "使用 7z 解压 $filename..."
                    7z x "$output_file" -o"$dir" -y || {
                        log_error "解压 $filename 失败"
                        continue
                    }
                    rm -rf $output_file
                else
                    case "$filename" in
                    *.tar.xz)
                        log_info "解压 $filename..."
                        tar -xJf "$output_file" -C "$dir" || {
                            log_error "解压 $filename 失败"
                            continue
                        }
                        rm -rf $output_file
                        ;;
                    *.xz)
                        log_info "解压 $filename..."
                        xz -d "$output_file" || {
                            log_error "解压 $filename 失败"
                            continue
                        }
                        rm -rf $output_file
                        ;;
                    *.gz)
                        log_info "解压 $filename..."
                        gzip -d "$output_file" || {
                            log_error "解压 $filename 失败"
                            continue
                        }
                        rm -rf $output_file
                        ;;
                    *)
                        log_warning "无法识别的压缩格式: $filename，跳过解压"
                        ;;
                    esac
                fi
                log_success "解压 $filename 完成"
            else
                log_info "跳过解压 $filename (deb 或 whl 文件)"
            fi
        fi

    done
    if [[ "$found_match" == false ]]; then
        log_error "没有找到匹配的模块: $module (OS: $item_os, Arch: $item_arch)"
        return 1
    fi
    log_success "所有下载和解压操作完成"
}
parse_patch_args() {
    local module=""
    local version=""
    local use_latest=false
    local os=""
    local arch=""
    local output_dir="${SCRIPT_WORK_DIR}/patched"
    local no_backup=false
    local auto_package=false
    local force=false

    while [[ $# -gt 0 ]]; do
        case $1 in
        -m | --module)
            module="$2"
            shift 2
            ;;
        -v | --version)
            version="$2"
            shift 2
            ;;
        -latest)
            use_latest=true
            shift
            ;;
        -os)
            os="$2"
            shift 2
            ;;
        -arch)
            arch="$2"
            shift 2
            ;;
        -o | --output)
            output_dir="$2"
            shift 2
            ;;
        -n | --no-backup)
            no_backup=true
            shift
            ;;
        -a | --auto-package)
            auto_package=true
            shift
            ;;
        -f | --force)
            force=true
            shift
            ;;
        *)
            log_error "无效的参数: $1"
            show_patch_usage
            exit 1
            ;;
        esac
    done

    if [ -z "$module" ]; then
        log_error "必须指定模块名称"
        show_patch_usage
        exit 1
    fi

    if [ "$use_latest" = true ] && [ -n "$version" ]; then
        log_error "不能同时指定版本和使用最新版本"
        show_patch_usage
        exit 1
    fi

    if [ "$use_latest" = false ] && [ -z "$version" ]; then
        log_error "必须指定版本或使用 -latest 选项"
        show_patch_usage
        exit 1
    fi

    # 如果使用最新版本,获取最新版本号
    if [ "$use_latest" = true ]; then
        version=$(get_latest_frida_version)
        log_info "使用最新版本: $version"
    fi

    # 执行修补逻辑
    patch_frida_module "$module" "$version" "$os" "$arch" "$output_dir" "$keep_source" "$auto_package" "$force"
}

patch_frida_module() {
    local module="$1"
    local version="$2"
    local os="$3"
    local arch="$4"
    local output_dir="$5"
    local keep_source="$6"
    local auto_package="$7"
    local force="$8"

    log_info "准备下载模块: $module, 版本: $version, OS: $os, 架构: $arch, 输出目录: $output_dir, 保留源码: $keep_source, 自动打包: $auto_package, 强制下载: $force"

    # 首先下载模块
    download_frida_module "$version" false "$module" false "$output_dir" false "$os" "$arch" "$force"

    # 获取下载的文件路径
    local downloaded_file=$(find "$output_dir" -name "${module}*" -type f | head -n 1)
    if [ -z "$downloaded_file" ]; then
        log_error "无法找到下载的模块文件"
        return 1
    fi

    log_info "正在修补文件: $downloaded_file"

    # 编译hexreplace工具
    (cd $SCRIPT_WORK_DIR/hexreplace && go build -o $SCRIPT_WORK_DIR/build/hexreplace && cd ..) || {
        log_error "编译 hexreplace 工具失败"
        return 1
    }
    chmod +x $SCRIPT_WORK_DIR/build/hexreplace

    # 生成新的Frida名称（如果未指定则提示进行配置： config set frida-name ）
    if [ -z "$FRIDA_NAME" ]; then
        log_error "未指定 Frida 魔改名，请使用 config set frida-name 命令指定"
        read -p "请输入本次所采用的 Frida 魔改名: " value
        if [[ "$value" =~ ^[a-zA-Z]{5}$ ]]; then
            FRIDA_NAME="$value"
            log_success "Frida 魔改名已设置为: $FRIDA_NAME"
        else
            log_error "无效的 Frida 魔改名: $value"
            log_info "Frida 魔改名必须是恰好 5 个字母（a-z 或 A-Z）"
            return 1
        fi
    else
        log_info "使用指定的 Frida 魔改名: $FRIDA_NAME"
    fi

    # 修补二进制文件
    local patched_file="${output_dir}/${module}_${FRIDA_NAME}"
    ($SCRIPT_WORK_DIR/build/hexreplace "$downloaded_file" "$FRIDA_NAME" "$patched_file") || {
        log_error "修改 ${module} 二进制失败"
        return 1
    }

    # 设置权限
    sudo chmod +x "$patched_file"
    sudo chown root:wheel "$patched_file"

    log_success "模块修补完成: $patched_file"

    # 处理源文件
    if [ "$no_backup" = true ]; then
        rm -f "$downloaded_file"
        log_info "已删除源文件"
    fi

    # 自动打包
    if [ "$auto_package" = true ]; then
        log_info "正在自动打包修补后的模块..."

        # 获取原始文件的扩展名
        local original_extension="${downloaded_file##*.}"
        local packed_file="${patched_file}.${original_extension}"

        # 使用7z进行压缩
        if command -v 7z &>/dev/null; then
            case "$original_extension" in
            xz | gz | zip | tar | bz2 | 7z)
                7z a -t"$original_extension" "$packed_file" "$patched_file" >/dev/null || {
                    log_error "使用 7z 压缩失败"
                    return 1
                }
                ;;
            *)
                # 对于未知格式,使用gzip压缩
                gzip -c "$patched_file" >"${patched_file}.gz" || {
                    log_error "使用 gzip 压缩失败"
                    return 1
                }
                packed_file="${patched_file}.gz"
                log_warning "未知的压缩格式: ${original_extension}, 使用 gzip 压缩"
                ;;
            esac
        else
            # 如果7z不可用,退回到使用gzip
            gzip -c "$patched_file" >"${patched_file}.gz" || {
                log_error "使用 gzip 压缩失败"
                return 1
            }
            packed_file="${patched_file}.gz"
            log_warning "7z 不可用,使用 gzip 压缩"
        fi

        if [ "$packed_file" != "$patched_file" ]; then
            log_success "模块已打包: $packed_file"
            # 如果不保留源文件，删除未压缩的修补文件
            if [ "$no_backup" = true ]; then
                rm -f "$patched_file"
                log_info "已删除未压缩的修补文件"
            fi
        fi
    fi

    return 0
}
# 用户确认函数
confirm_execution() {
    if [ "$AUTO_CONFIRM" = "true" ]; then
        log_warning "自动确认模式：用户已同意免责声明和sudo权限使用。"
        return 0
    fi

    log_color $COLOR_PURPLE "$DISCLAIMER"
    log_warning "本脚本将会要求使用sudo权限以修改文件权限。"

    read -p "您是否同意以上免责声明并允许使用sudo权限？(y/N) " response
    case "$response" in
    [yY][eE][sS] | [yY])
        return 0
        ;;
    *)
        log_info "用户不同意，操作已取消"
        exit 0
        ;;
    esac
}
confirm_modify_frida_tools() {
    if [ "$AUTO_CONFIRM" = "true" ]; then
        log_warning "自动确认模式：用户已同意自动修改本地 frida-tools。"
        return 0
    fi
    log_color $COLOR_PURPLE "本脚本将自动修改本地 frida-tools，以适配魔改版本的 Frida。（跳过 frida-tools 魔改。某些功能可能无法使用，建议修改）"
    read -p "您是否同意？(y/N) " response
    case "$response" in
    [yY][eE][sS] | [yY])
        return 0
        ;;
    *)
        log_info "用户不同意，操作已取消"
        return 1
        ;;
    esac
}
check_dependencies() {
    local missing_tools=()
    local tools=("xcode-select" "brew" "git" "jq" "dpkg-deb" "go" "python3" "7z" "curl" "xz" "gzip")

    for tool in "${tools[@]}"; do
        if ! command -v $tool &>/dev/null; then
            missing_tools+=("$tool")
            log_warning "$COLOR_YELLOW$tool$COLOR_RESET ${COLOR_PURPLE}未找到"
        else
            log_success "$COLOR_YELLOW$tool$COLOR_RESET ${COLOR_SKYBLUE}已安装"
        fi
    done

    # 检查 Frida 工具
    if ! check_frida_tool; then
        missing_tools+=("frida-tools")
        log_warning "${COLOR_YELLOW}frida-tools${COLOR_RESET} ${COLOR_PURPLE}未找到"
    else
        log_success "${COLOR_YELLOW}frida-tools$COLOR_RESET ${COLOR_SKYBLUE}已安装"
    fi

    if [ ${#missing_tools[@]} -eq 0 ]; then
        log_success "所有依赖已安装"
        return 0
    else
        log_warning "以下工具未安装: ${missing_tools[*]}"
        return 1
    fi

}

install_dependencies() {
    log_warning "正在安装缺失的依赖..."
    confirm_execution

    check_and_install_tool "xcode-select" "xcode-select --install"
    check_and_install_tool "brew" "curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | bash"
    check_and_install_tool "git" "brew install git"
    check_and_install_tool "jq" "brew install jq"
    check_and_install_tool "dpkg-deb" "brew install dpkg"
    check_and_install_tool "go" "brew install go"
    check_and_install_tool "python3" "brew install python3"
    check_and_install_tool "7z" "brew install p7zip"
    check_and_install_tool "curl" "brew install curl"
    check_and_install_tool "xz" "brew install xz"
    check_and_install_tool "gzip" "brew install gzip"

    install_frida_tools

    log_success "依赖安装完成"
}

setup_environment() {
    log_info "检查系统依赖..."
    if ! check_dependencies; then
        install_dependencies
    fi

    log_success "环境设置完成"
}
check_frida_tool() {
    if command -v frida-ps >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}
install_frida_tools() {
    log_info "正在安装 frida-tools..."

    local pip_commands=("pip" "pip3")
    for pip_cmd in "${pip_commands[@]}"; do
        if command -v "$pip_cmd" >/dev/null 2>&1; then
            if $pip_cmd install --user frida-tools --force-reinstall; then
                log_success "frida-tools 安装成功"
                return 0
            else
                log_error "使用 $pip_cmd 安装 frida-tools 失败"
            fi
        fi
    done

    log_error "无法安装 frida-tools。请确保 pip 已正确安装并且网络连接正常。"
    return 1
}
check_and_install_tool() {
    local tool=$1
    local install_cmd=$2

    if ! command -v $tool &>/dev/null; then
        log_warning "$tool 未找到，正在安装..."
        eval $install_cmd || {
            log_error "安装 $tool 失败"
            exit 1
        }
    else
        log_success "$tool 已安装"
    fi
}
# 函数：下载 Frida
download_frida() {
    local arch=$1
    local fridaver=$2
    local clean=$3
    local filename="frida_${fridaver}_iphoneos-${arch}.deb"
    local output="${SCRIPT_WORK_DIR}/build/$filename"
    # 如果本地文件存在
    if [ -f "$output" ]; then
        if [ "$clean" = "true" ]; then
            log_warning "清理旧文件: $output"
            rm -f $output
        fi
    fi

    if [ ! -f "$output" ]; then
        local url="https://github.com/frida/frida/releases/download/${fridaver}/${filename}"
        echo $url
        download_with_progress "$url" "$output"
    else
        log_warning "本地存在 $filename"
    fi

    log_success "下载 $filename 完成"
}
# 新增函数：删除目录内的所有 .DS_Store 文件
remove_ds_store() {
    local dir=$1
    log_success "正在删除 $dir 中的 .DS_Store 文件..."
    find "$dir" -name ".DS_Store" -type f -delete
    log_info ".DS_Store 文件删除完成"
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
        log_error "错误: plist 文件不存在: $plist"
        return 1
    fi

    log_success "正在修改 plist 文件: $plist"
    log_info "FRIDA_NAME: $FRIDA_NAME"
    log_info "FRIDA_SERVER_PORT: $FRIDA_SERVER_PORT"

    # 检查变量是否为空
    if [ -z "$FRIDA_NAME" ] || [ -z "$FRIDA_SERVER_PORT" ]; then
        log_error "错误: FRIDA_NAME 或 FRIDA_SERVER_PORT 为空"
        return 1
    fi

    # 使用 -e 选项为每个替换操作创建单独的 sed 命令
    sed -i '' \
        -e 's/re\.frida\.server/re.'"${FRIDA_NAME}"'.server/g' \
        -e 's/frida-server/'"${FRIDA_NAME}"'/g' \
        -e 's@</array>@\t<string>-l</string>\n\t\t<string>0.0.0.0:'"${FRIDA_SERVER_PORT}"'</string>\n\t</array>@g' \
        "$plist"

    if [ $? -ne 0 ]; then
        log_error "错误: sed 命令执行失败"
        return 1
    fi

    log_success "plist 文件修改完成"

    # 收到一个 arm64e 的版本，对它做了适配：
    # 检查 usr/sbin/frida-server-wrapper 和 usr/sbin/frida-server.ent
    # 存在则更名 ${FRIDA_NAME}-server-wrapper 和 usr/sbin/${FRIDA_NAME}-server.ent
    # 打开 ${FRIDA_NAME}-server-wrapper ，进行替换
    # exec /usr/sbin/frida-server $@ 替换为 exec /usr/sbin/${FRIDA_NAME}-server $@
    # 打开 usr/sbin/${FRIDA_NAME}-server.ent，进行替换
    # <string>re.frida.Server</string> 替换为 <string>re.${FRIDA_NAME}.Server</string>
    # 修改2个文件权限
    # local frida_server_wrapper_file="${build}/usr/sbin/frida-server-wrapper"
    # local new_frida_server_wrapper_file="${build}/usr/sbin/${FRIDA_NAME}-server-wrapper"
    # local frida_server_ent_file="${build}/usr/sbin/frida-server.ent"
    # local new_frida_server_ent_file="${build}/usr/sbin/${FRIDA_NAME}-server.ent"

    # if [ -f "$frida_server_wrapper_file" ]; then
    #     log_success "正在修改 frida-server-wrapper 文件: $frida_server_wrapper_file"
    #     sed -i '' 's/exec \/usr\/sbin\/frida-server/exec \/usr\/sbin\/'"${FRIDA_NAME}"'-server/g' "$frida_server_wrapper_file"
    #     if [ $? -ne 0 ]; then
    #         log_error "错误: 修改 frida-server-wrapper 文件失败"
    #         return 1
    #     fi
    #     log_success "frida-server-wrapper 文件修改完成"
    #     mv "$frida_server_wrapper_file" "$new_frida_server_wrapper_file"
    #     if [ $? -ne 0 ]; then
    #         log_error "错误: 重命名 frida-server-wrapper 文件失败"
    #         return 1
    #     fi
    #     log_success "frida-server-wrapper 文件已重命名为: $new_frida_server_wrapper_file"
    #     sudo chown root:wheel $new_frida_server_wrapper_file
    #     if [ $? -ne 0 ]; then
    #         log_error "错误: 修改 frida-server-wrapper 文件权限失败"
    #         return 1
    #     fi
    #     log_success "frida-server-wrapper 文件权限修改完成"
    # else
    #     log_warning "警告: frida-server-wrapper 文件不存在: $frida_server_wrapper_file"
    # fi

    # if [ -f "$frida_server_ent_file" ]; then
    #     log_success "正在修改 frida-server.ent 文件: $frida_server_ent_file"
    #     sed -i '' 's/<string>re\.frida\.Server/<string>re.'"${FRIDA_NAME}"'.Server/g' "$frida_server_ent_file"
    #     if [ $? -ne 0 ]; then
    #         log_error "错误: 修改 frida-server.ent 文件失败"
    #         return 1
    #     fi
    #     log_success "frida-server.ent 文件修改完成"
    #     mv "$frida_server_ent_file" "$new_frida_server_ent_file"
    #     if [ $? -ne 0 ]; then
    #         log_error "错误: 重命名 frida-server.ent 文件失败"
    #         return 1
    #     fi
    #     log_success "frida-server.ent 文件已重命名为: $new_frida_server_ent_file"
    #     sudo chown root:wheel $new_frida_server_ent_file
    #     if [ $? -ne 0 ]; then
    #         log_error "错误: 修改 frida-server.ent 文件权限失败"
    #         return 1
    #     fi
    #     log_success "frida-server.ent 文件权限修改完成"
    # else
    #     log_warning "警告: frida-server.ent 文件不存在: $frida_server_ent_file"
    # fi

    # log_success "frida-server-wrapper 和 frida-server.ent 文件修改完成"

    # 重命名 plist 文件
    local new_plist
    if [ "$arch" = "arm64" ]; then
        new_plist="${build}/var/jb/Library/LaunchDaemons/re.${FRIDA_NAME}.server.plist"
    else
        new_plist="${build}/Library/LaunchDaemons/re.${FRIDA_NAME}.server.plist"
    fi

    mv "$plist" "$new_plist"
    sudo chown root:wheel $new_plist

    if [ $? -ne 0 ]; then
        log_error "错误: 重命名 plist 文件失败"
        return 1
    fi

    log_success "plist 文件已重命名为: $new_plist"
}

modify_debian_files() {
    local build=$1
    local arch=$2
    local debian_dir

    debian_dir="${build}/DEBIAN"

    log_success "正在修改 DEBIAN 文件夹中的文件: $debian_dir"
    log_info "FRIDA_NAME: $FRIDA_NAME"

    # 检查变量是否为空
    if [ -z "$FRIDA_NAME" ]; then
        log_error "错误: FRIDA_NAME 为空"
        return 1
    fi

    # 修改 control 文件
    local control_file="${debian_dir}/control"
    if [ -f "$control_file" ]; then
        log_info "修改 control 文件"
        sed -i '' 's/Package: re\.frida\.server/Package: re.'"${FRIDA_NAME}"'.server/g' "$control_file"
        if [ $? -ne 0 ]; then
            log_error "错误: 修改 control 文件失败"
            return 1
        fi
    else
        log_warning "警告: control 文件不存在: $control_file"
    fi
    sudo chown root:wheel $control_file

    # 修改 extrainst_ 文件
    local extrainst_file="${debian_dir}/extrainst_"
    if [ -f "$extrainst_file" ]; then
        log_info "修改 extrainst_ 文件"
        if [ "$arch" = "arm64" ]; then
            sed -i '' 's@launchcfg=/var/jb/Library/LaunchDaemons/re\.frida\.server\.plist@launchcfg=/var/jb/Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$extrainst_file"
        else
            sed -i '' 's@launchcfg=/Library/LaunchDaemons/re\.frida\.server\.plist@launchcfg=/Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$extrainst_file"
        fi
        if [ $? -ne 0 ]; then
            log_error "错误: 修改 extrainst_ 文件失败"
            return 1
        fi
    else
        log_warning "警告: extrainst_ 文件不存在: $extrainst_file"
    fi
    sudo chown root:wheel $extrainst_file

    # 修改 prerm 文件
    local prerm_file="${debian_dir}/prerm"
    if [ -f "$prerm_file" ]; then
        log_info "修改 prerm 文件"
        if [ "$arch" = "arm64" ]; then
            sed -i '' 's@launchctl unload /var/jb/Library/LaunchDaemons/re\.frida\.server\.plist@launchctl unload /var/jb/Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$prerm_file"
        else
            sed -i '' 's@launchctl unload /Library/LaunchDaemons/re\.frida\.server\.plist@launchctl unload /Library/LaunchDaemons/re.'"${FRIDA_NAME}"'.server.plist@g' "$prerm_file"
        fi
        if [ $? -ne 0 ]; then
            log_error "错误: 修改 prerm 文件失败"
            return 1
        fi
    else
        log_warning "警告: prerm 文件不存在: $prerm_file"
    fi
    sudo chown root:wheel $prerm_file

    log_success "DEBIAN 文件夹中的文件修改完成"
}

modify_binary() {
    local build=$1
    local arch=$2
    local frida_server_path
    local new_path
    local frida_dylib_file
    local new_dylib_file
    local dylib_folder
    local new_dylib_folder

    if [ "$arch" = "arm64" ]; then
        frida_server_path="${build}/var/jb/usr/sbin/frida-server"
        new_path="${build}/var/jb/usr/sbin/${FRIDA_NAME}"
        frida_dylib_file="${build}/var/jb/usr/lib/frida/frida-agent.dylib"
        new_dylib_file="${build}/var/jb/usr/lib/frida/${FRIDA_NAME}-agent.dylib"
        dylib_folder="${build}/var/jb/usr/lib/frida"
        new_dylib_folder="${build}/var/jb/usr/lib/${FRIDA_NAME}"
    else
        frida_server_path="${build}/usr/sbin/frida-server"
        new_path="${build}/usr/sbin/${FRIDA_NAME}"
        frida_dylib_file="${build}/usr/lib/frida/frida-agent.dylib"
        new_dylib_file="${build}/usr/lib/frida/${FRIDA_NAME}-agent.dylib"
        dylib_folder="${build}/usr/lib/frida"
        new_dylib_folder="${build}/usr/lib/${FRIDA_NAME}"
    fi
    log_success "正在修改二进制文件: $frida_server_path"
    if [ ! -f "$frida_server_path" ]; then
        log_error "错误: frida-server 文件不存在于路径: $frida_server_path"
        return 1
    fi

    (cd $SCRIPT_WORK_DIR/hexreplace && go build -o $SCRIPT_WORK_DIR/build/hexreplace && cd ..) || {
        log_error "编译 hexreplace 工具失败"
        return 1
    }
    chmod +x $SCRIPT_WORK_DIR/build/hexreplace
    ($SCRIPT_WORK_DIR/build/hexreplace "$frida_server_path" "$FRIDA_NAME" "$new_path") || {
        log_error "修改 frida-server 二进制失败"
        return 1
    }
    rm -rf $frida_server_path
    # 确保新文件有执行权限
    sudo chmod +x $new_path
    sudo chown root:wheel $new_path

    ($SCRIPT_WORK_DIR/build/hexreplace $frida_dylib_file $FRIDA_NAME $new_dylib_file) || {
        log_error "修改 frida-agent.dylib 失败"
        return 1
    }
    rm -rf $frida_dylib_file
    # 确保新文件有执行权限
    sudo chmod +x $new_dylib_file
    sudo chown root:wheel $new_dylib_file

    # 修改dylib目录
    if ! mv "$dylib_folder" "$new_dylib_folder"; then
        log_error "重命名 dylib 目录失败"
        return 1
    fi
    log_success "二进制文件修改完成"
    return 0
}

# 函数：重新打包 deb 文件
repackage_deb() {
    local build=$1
    local output_filename=$2
    # 在打包之前删除 .DS_Store 文件
    remove_ds_store "$build"
    # 打包
    dpkg-deb -b "$build" "$output_filename" || {
        log_error "打包 $output_filename 失败"
        exit 1
    }

    rm -rf "$build"

    log_success "重新打包 $output_filename 完成"
}

find_frida_path() {
    local pip_commands=("pip3" "pip")
    local frida_path=""

    for pip_cmd in "${pip_commands[@]}"; do
        if command -v "$pip_cmd" >/dev/null 2>&1; then
            frida_path=$("$pip_cmd" show frida 2>/dev/null | grep "Location:" | cut -d " " -f 2-)
            if [ -n "$frida_path" ]; then
                frida_path="${frida_path}/frida"
                if [ -d "$frida_path" ]; then
                    echo "$frida_path"
                    return 0
                fi
            fi
        fi
    done

    # 如果通过 pip 无法找到，尝试常见路径
    local frida_paths=(
        "/usr/local/lib/python*/site-packages/frida"
        "/usr/lib/python*/site-packages/frida"
        "$HOME/.local/lib/python*/site-packages/frida"
        "$HOME/Library/Python/*/lib/python/site-packages/frida"
        "$HOME/anaconda*/lib/python*/site-packages/frida"
        "/opt/homebrew/lib/python*/site-packages/frida"
        "/Library/Frameworks/Python.framework/Versions/*/lib/python*/site-packages/frida"
        "/Applications/Frida.app/Contents/Resources/lib/python*/site-packages/frida"
    )

    for path_pattern in "${frida_paths[@]}"; do
        for path in $path_pattern; do
            if [ -d "$path" ]; then
                echo "$path"
                return 0
            fi
        done
    done

    log_error "无法找到 frida-tools 路径。请确保 frida-tools 已正确安装。"
    return 1
}
generate_random_name() {
    cat /dev/urandom | env LC_CTYPE=C tr -dc 'a-z' | fold -w 5 | head -n 1
}
patch_frida_tools() {
    local local_frida_name="$1"

    log_info "开始给 frida-tools 打补丁..."

    # 如果未提供名称，尝试使用配置的名称或生成随机名称
    if [ -z "$local_frida_name" ]; then
        if [ -n "$FRIDA_NAME" ]; then
            local_frida_name="$FRIDA_NAME"
            log_info "使用配置的魔改名: $local_frida_name"
        else
            local_frida_name=$(generate_random_name)
            log_info "生成随机魔改名: $local_frida_name"
        fi
    fi

    # 检查新名称是否有效
    if [[ ! "$local_frida_name" =~ ^[a-zA-Z]{5}$ ]]; then
        log_error "无效的魔改名: $local_frida_name"
        log_info "魔改名必须是恰好 5 个字母（a-z 或 A-Z）"
        return 1
    fi

    modify_frida_tools "$local_frida_name"
}

restore_frida_tools() {
    log_info "开始恢复 frida-tools 到原版..."
    local python_cmd=$(get_python_cmd)
    if [ -z "$python_cmd" ]; then
        log_error "未找到 Python 解释器"
        return 1
    fi

    local frida_tools_path=$($python_cmd -c "import os, frida; print(os.path.dirname(frida.__file__))" 2>/dev/null)
    if [ $? -ne 0 ]; then
        log_error "执行 Python 命令失败，请确保 frida 已正确安装"
        return 1
    fi
    # 恢复 Python 库文件
    local pylib_backup=$(ls $frida_tools_path/*.so.fridare 2>/dev/null)
    local pylib=$(ls $frida_tools_path/*.so 2>/dev/null)

    if [ -z "$pylib_backup" ]; then
        log_warning "未找到 Python 库文件的备份"
        return 1
    else
        log_info "正在恢复 Python 库文件: $pylib"
        mv "$pylib_backup" "$pylib"
    fi

    # 恢复 core.py 文件
    local core_py="$frida_tools_path/core.py"
    if [ -f "$core_py.fridare" ]; then
        log_info "正在恢复 core.py 文件: $core_py"
        mv "$core_py.fridare" "$core_py"
    else
        log_warning "未找到 core.py 文件的备份"
    fi

    rm -rf "$frida_tools_path/__pycache__"
    log_success "frida-tools 已恢复到原版"
}
# 函数：修订frida-tools
modify_frida_tools() {
    local local_frida_name="$1"

    local python_cmd=$(get_python_cmd)
    if [ -z "$python_cmd" ]; then
        log_error "未找到 Python 解释器"
        return 1
    fi

    local pylib_path=$($python_cmd -c "import os, frida; print(os.path.dirname(frida.__file__))" 2>/dev/null)
    if [ $? -ne 0 ]; then
        log_error "执行 Python 命令失败，请确保 frida 已正确安装"
        return 1
    fi
    local pylib=$(ls $pylib_path/*.so 2>/dev/null)
    if [ -z "$pylib" ]; then
        log_error "未找到 frida Python 库"
        return 1
    fi

    if [ ! -f "$pylib.fridare" ]; then
        cp "$pylib" "$pylib.fridare"
        log_info "创建备份: $pylib.fridare"
    else
        log_info "备份已存在: $pylib.fridare"
    fi

    log_info "Python 库文件: $pylib"
    log_info "Frida 名称: $local_frida_name"

    $SCRIPT_WORK_DIR/build/hexreplace "$pylib" "$local_frida_name" "test.so" || {
        log_error "修改 frida Python 库失败"
        return 1
    }

    rm -f "$pylib"
    rm -rf "$pylib_path/__pycache__"
    mv test.so "$pylib"
    chmod 755 "$pylib"

    $python_cmd -c "
import os, frida, shutil, re, sys

def modify_core_py(frida_name):
    p = os.path.join(os.path.dirname(frida.__file__), 'core.py')
    b = p + '.fridare'
    if not os.path.exists(b):
        print(f'Creating backup: {b}')
        shutil.copy2(p, b)
    else:
        print(f'Backup already exists: {b}')
    try:
        with open(p, 'r') as f:
            lines = f.readlines()
        replaced = False
        for i, line in enumerate(lines):
            matches = re.finditer(r'\"([^\"]{5}):rpc\"', line)
            for match in matches:
                old = match.group(1)
                new = frida_name[:5].ljust(5)
                line = line.replace(f'\"{old}:rpc\"', f'\"{new}:rpc\"')
                print(f'Line {i+1}: Replaced \"{old}:rpc\" with \"{new}:rpc\"')
                replaced = True
            lines[i] = line
        if replaced:
            with open(p, 'w') as f:
                f.writelines(lines)
            print('Replacement complete')
        else:
            print('No matching pattern found, no changes made')
    except Exception as e:
        print(f'Error: {e}')
        if os.path.exists(b):
            print('Restoring from backup')
            shutil.copy2(b, p)
        else:
            print('No backup found to restore from')
        sys.exit(1)

modify_core_py('$local_frida_name')
" || {
        log_error "修改 core.py 失败"
        return 1
    }

    log_success "frida-tools 修改完成"
    return 0
}
get_absolute_path() {
    local relative_path="$1"
    local absolute_path=""

    # 如果是相对路径，添加当前目录
    if [[ "$relative_path" != /* ]]; then
        relative_path="$PWD/$relative_path"
    fi

    # 规范化路径
    local oldIFS="$IFS"
    IFS='/'
    local path_parts=($relative_path)
    local new_path=()

    for part in "${path_parts[@]}"; do
        case "$part" in
        "" | ".") ;;
        "..")
            if [ ${#new_path[@]} -ne 0 ]; then
                unset 'new_path[${#new_path[@]}-1]'
            fi
            ;;
        *)
            new_path+=("$part")
            ;;
        esac
    done

    IFS="$oldIFS"
    absolute_path="/${new_path[*]}"
    absolute_path="${absolute_path// //}"

    echo "$absolute_path"
}
move_file() {
    local source_file="$1"
    local target_dir="$2"
    mv "$OUTPUT_FIsource_fileLENAME" "$target_dir" 2>&1 | grep -v "are identical" || true
}
build_frida() {
    local clean=$1
    local local_deb=$2
    local local_arch=$3
    # 检查并设置 Frida 版本
    local use_local=false
    if [ -n "$local_deb" ]; then
        # 首先检查文件是否存在
        if [ ! -f "$local_deb" ]; then
            log_error "本地文件不存在: $local_deb"
            exit 1
        fi
        # 获取 local_deb 的绝对路径
        if command -v realpath >/dev/null 2>&1; then
            local_deb=$(realpath "$local_deb")
        else
            # 如果 realpath 不可用，使用自定义函数
            local_deb=$(get_absolute_path "$local_deb")
        fi
        if [ ! -f "$local_deb" ]; then
            log_error "本地文件不存在: $local_deb"
            exit 1
        fi
        log_info "使用本地文件: $local_deb"
        use_local=true
    else
        if [ -z "$FRIDA_VERSION" ]; then
            log_error "未指定 Frida 版本"
            show_build_usage
            exit 1
        fi
    fi

    log_info "使用 Frida 服务器端口: $FRIDA_SERVER_PORT"
    [ -n "$CURL_PROXY" ] && log_info "HTTP 代理：${CURL_PROXY}"
    [ "$AUTO_CONFIRM" = "true" ] && log_info "自动确认：已启用"

    log_warning "期间可能会要求输入 sudo 密码，用于修改文件权限"
    log_color "${COLOR_GREEN}构建目录：${COLOR_RESET} $SCRIPT_WORK_DIR/build"
    log_color "${COLOR_GREEN}输出目录：${COLOR_RESET} $SCRIPT_WORK_DIR/dist"

    # 确认执行
    if [ "$AUTO_CONFIRM" != "true" ]; then
        confirm_execution
    fi

    log_info "开始构建 Frida..."
    # 检查并安装 dpkg

    if ! check_dependencies; then
        log_error "依赖检查失败"
        log_success "请使用 './$0 setup' 命令安装依赖"
        exit 1
    fi

    # 如果 FRIDA_NAME 为空，生成一个新的
    if [ -z "$FRIDA_NAME" ]; then
        FRIDA_NAME=$(generate_random_name)
        if [[ ! "$FRIDA_NAME" =~ ^[a-z]{5}$ ]]; then
            log_error "无法生成有效的 Frida 魔改名"
            exit 1
        fi
    fi

    local architectures=("arm" "arm64")
    for arch in "${architectures[@]}"; do
        local input_file
        if [ "$use_local" = "true" ]; then
            input_file="$local_deb"
            OUTPUT_FILENAME="${local_deb}_${FRIDA_NAME}_tcp.deb"
            if [ "$local_arch" = "arm64e" ]; then
                arch="arm"
            fi
            log_info "使用本地文件: $input_file"
        else
            input_file="${SCRIPT_WORK_DIR}/build/frida_${FRIDA_VERSION}_iphoneos-${arch}.deb"
            OUTPUT_FILENAME="${SCRIPT_WORK_DIR}/build/frida_${FRIDA_VERSION}_iphoneos-${arch}_${FRIDA_NAME}_tcp.deb"
            download_frida $arch $FRIDA_VERSION $clean
        fi

        BUILD_DIR="${SCRIPT_WORK_DIR}/build/frida_build_${arch}"
        rm -rf "$BUILD_DIR"

        dpkg-deb -R "${input_file}" "${BUILD_DIR}"

        log_cinfo $COLOR_GREEN "正在修改 Frida ${COLOR_PURPLE}${FRIDA_VERSION}${COLOR_RESET} 版本 (${COLOR_SKYBLUE}${arch}${COLOR_RESET})"
        modify_launch_daemon "$BUILD_DIR" "$arch"
        modify_debian_files "$BUILD_DIR" "$arch"
        modify_binary "$BUILD_DIR" "$arch"

        repackage_deb "$BUILD_DIR" "$OUTPUT_FILENAME"

        mkdir -p $SCRIPT_WORK_DIR/dist

        mv "$OUTPUT_FILENAME" $SCRIPT_WORK_DIR/dist/ 2>&1 | grep -v "are identical" || true

        log_success "Frida ${FRIDA_VERSION} 版本 (${arch}) 修改完成"

        log_info "新版本名：${FRIDA_NAME}"
        log_info "请使用新版本名：${FRIDA_NAME} 进行调试"
        log_info "请使用端口：${FRIDA_SERVER_PORT} 进行调试"
        log_info "新版本 deb 文件：$SCRIPT_WORK_DIR/dist/${OUTPUT_FILENAME}"
        log_info "-------------------------------------------------"
        log_info "iPhone 安装："
        log_info "scp dist/${OUTPUT_FILENAME} root@<iPhone-IP>:/var/root"
        log_info "ssh root@<iPhone-IP>"
        log_info "dpkg -i /var/root/${OUTPUT_FILENAME}"
        log_info "PC 连接："
        log_info "frida -U -f com.xxx.xxx -l"
        log_info "frida -H <iPhone-IP>:${FRIDA_SERVER_PORT} -f com.xxx.xxx --no-pause"
        log_info "-------------------------------------------------"

        if [ -n "$local_deb" ]; then
            return 0 # 如果是本地文件，只处理一次
        fi
    done

    # 确认执行
    if [ "$AUTO_CONFIRM" != "true" ]; then
        # 不同意返回0
        if ! confirm_modify_frida_tools; then
            log_success "frida-tools 未修改。"
            exit 0
        fi
    fi
    modify_frida_tools "$FRIDA_NAME"

}
initialize_config() {
    SCRIPT_WORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    log_color $COLOR_GREEN "工作目录：$SCRIPT_WORK_DIR"
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "# Fridare Configuration File" >"$CONFIG_FILE"
        echo "FRIDA_SERVER_PORT=$DEF_FRIDA_SERVER_PORT" >>"$CONFIG_FILE"
        echo "CURL_PROXY=" >>"$CONFIG_FILE"
        echo "AUTO_CONFIRM=$DEF_AUTO_CONFIRM" >>"$CONFIG_FILE"
        echo "FRIDA_NAME=" >>"$CONFIG_FILE"
        echo "FRIDA_MODULES=(" >>"$CONFIG_FILE"
        echo ")" >>"$CONFIG_FILE"
    fi

    # 检查并创建 build 和 dist 目录
    if [ ! -d "${SCRIPT_WORK_DIR}/build" ]; then
        mkdir -p ${SCRIPT_WORK_DIR}/build
    fi
    if [ ! -d "${SCRIPT_WORK_DIR}/dist" ]; then
        mkdir -p ${SCRIPT_WORK_DIR}/dist
    fi

}
is_conda_env() {
    # 检查 CONDA_PREFIX 环境变量是否存在
    [ -n "$CONDA_PREFIX" ]
}
get_python_cmd() {
    if is_conda_env; then
        echo "$CONDA_PREFIX/bin/python"
    else
        for cmd in python3 python; do
            if command -v $cmd >/dev/null 2>&1; then
                echo $cmd
                return
            fi
        done
    fi
}
#创建了一个后台进程，其目的是保持 sudo 权限活跃
sudo_keep_alive() {
    while true; do
        sudo -n true
        sleep 60
        kill -0 "$$" || exit
    done 2>/dev/null &
    SUDO_KEEP_ALIVE_PID=$!
}

# 在脚本结束时清理
cleanup() {
    if [ -n "$SUDO_KEEP_ALIVE_PID" ]; then
        kill $SUDO_KEEP_ALIVE_PID
    fi
}
get_golang_info() {
    if command -v go >/dev/null 2>&1; then
        local go_version=$(go version 2>&1)
        local go_path=$(go env GOPATH 2>/dev/null)
        echo "$go_version:$go_path"
    else
        echo "Not installed"
    fi
}
log_environment_info() {
    log_skyblue "环境信息:"

    # Python 环境信息
    if is_conda_env; then
        log_skyblue "  Conda 环境: $CONDA_PREFIX"
    else
        log_skyblue "  使用系统 Python 环境"
    fi
    local python_cmd=$(get_python_cmd)
    log_skyblue "  Python 路径: $python_cmd"
    log_skyblue "  Python 版本: $($python_cmd --version 2>&1)"

    # Frida 信息
    local frida_version=$($python_cmd -c 'import frida; print(frida.__version__)' 2>/dev/null)
    if [ -n "$frida_version" ]; then
        log_skyblue "  Frida 版本: $frida_version"
        log_skyblue "  Frida 路径: $($python_cmd -c 'import os; import frida; print(os.path.dirname(frida.__file__))' 2>/dev/null)"
    else
        log_warning "  Frida 未安装或无法检测"
    fi

    # Golang 环境信息
    local golang_info=$(get_golang_info)
    if [ "$golang_info" != "Not installed" ]; then
        IFS=':' read -r go_version go_path <<<"$golang_info"
        log_skyblue "  Golang 版本: $go_version"
        log_skyblue "  GOPATH: $go_path"
    else
        log_warning "  Golang 未安装或无法检测"
    fi

    # 操作系统信息
    log_skyblue "  操作系统: $(uname -s)"
    log_skyblue "  系统版本: $(uname -r)"

    echo # 空行，为了更好的可读性
}

log_config_info() {
    log_skyblue "配置信息:"
    log_skyblue "  FRIDA_SERVER_PORT: $FRIDA_SERVER_PORT"
    log_skyblue "  CURL_PROXY: $CURL_PROXY"
    log_skyblue "  AUTO_CONFIRM: $AUTO_CONFIRM"
    log_skyblue "  FRIDA_NAME: $FRIDA_NAME"
    echo # 空行，为了更好的可读性
}

version_compare() {
    local v1="$1"
    local v2="$2"

    # Remove 'v' if present
    v1="${v1#v}"
    v2="${v2#v}"

    # Split the versions into arrays
    IFS='.' read -r -a ver1 <<<"$v1"
    IFS='.' read -r -a ver2 <<<"$v2"

    # Compare each part of the version
    for ((i = 0; i < ${#ver1[@]} || i < ${#ver2[@]}; i++)); do
        local num1=$((${ver1[i]:-0}))
        local num2=$((${ver2[i]:-0}))
        if ((num1 > num2)); then
            echo ">" # v1 is greater
            return
        elif ((num1 < num2)); then
            echo "<" # v2 is greater
            return
        fi
    done
    echo "=" # versions are equal
}

check_version() {
    local is_install=$1
    local current_version="$VERSION"
    local repo_owner="suifei"
    local repo_name="fridare"
    local next="false"

    log_info "检查版本更新..."
    local releases_info=$(curl -s "https://api.github.com/repos/$repo_owner/$repo_name/releases")

    if [ -z "$releases_info" ]; then
        log_error "无法获取版本信息"
        return 1
    fi

    # 获取所有非预发布版本，并按版本号排序
    local versions=$(echo "$releases_info" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -rV)
    local latest_version=$(echo "$versions" | head -n1)

    if [ -z "$latest_version" ]; then
        log_error "无法获取最新版本信息"
        return 1
    fi

    local download_url=$(echo "$releases_info" | jq -r ".[] | select(.tag_name == \"$latest_version\") | .zipball_url")

    if [ -z "$download_url" ]; then
        log_error "无法获取下载链接"
        return 1
    fi

    current_version="${current_version#v}"
    latest_version="${latest_version#v}"

    result=$(version_compare "$current_version" "$latest_version")

    if [ "$result" = "=" ]; then
        log_success "当前版本 (${current_version}) 已是最新正式版本"
    elif [ "$result" = ">" ]; then
        log_success "当前版本 (${current_version}) 比最新正式版本 (${latest_version}) 更新"
    elif [ "$result" = "<" ]; then
        log_warning "发现新的正式版本：${latest_version}（当前版本：${current_version}）"
        echo "更新说明："
        echo "$releases_info" | jq -r ".[] | select(.tag_name == \"$latest_version\") | .body" | sed 's/^/  /'

        read -p "是否更新到最新正式版本？(y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "取消更新"
            return 0
        fi
        next="true"
    else
        log_error "版本比较出错"
    fi
    if [ "$next" = "true" ] || [ "$is_install" = "true" ]; then

        # 下载和更新过程保持不变
        log_info "开始下载最新正式版本..."
        local temp_dir=$(mktemp -d)
        local zip_file="$temp_dir/fridare-$latest_version.zip"
        local extract_dir="$temp_dir/extracted_files"

        download_with_progress "$download_url" "$zip_file" "Fridare 版本 ${latest_version}"

        log_info "解压文件..."
        if ! unzip -q "$zip_file" -d "$extract_dir"; then
            log_error "解压失败"
            rm -rf "$temp_dir"
            return 1
        fi

        log_info "更新本地文件..."
        local script_dir="$(dirname "$0")"
        local install_dir="${script_dir}/fridare"

        if [ "$is_install" = "true" ]; then
            # 创建目录
            mkdir -p "$install_dir"
            script_dir="$install_dir"
            log_success "文件夹 \"${script_dir}\" 已创建，请将此文件夹添加到 PATH 环境变量中"
            log_skyblue "  export PATH=\$PATH:\"${script_dir}\""
            # 加入 .bashrc 或者 .zshrc 的提示
            if [ -f "$HOME/.bashrc" ]; then
                log_skyblue "  echo \"export PATH=\$PATH:\\\"${script_dir}\\\"\" >> ~/.bashrc"
            fi
            if [ -f "$HOME/.zshrc" ]; then
                log_skyblue "  echo \"export PATH=\$PATH:\\\"${script_dir}\\\"\" >> ~/.zshrc"
            fi
        fi

        # 找到解压后的目录（应该只有一个）
        local update_dir=$(find "$extract_dir" -maxdepth 1 -type d | grep -v "^$extract_dir$" | head -n 1)

        if [ -z "$update_dir" ]; then
            log_error "无法找到更新文件目录"
            rm -rf "$temp_dir"
            return 1
        fi

        log_info "正在从 $update_dir 复制文件到 $script_dir"

        # 复制新文件到脚本目录
        if ! cp -R "$update_dir/"* "$script_dir/"; then
            log_error "复制新文件失败"
            rm -rf "$temp_dir"
            return 1
        fi

        # 删除旧文件
        find "$script_dir" -type f | while read file; do
            if [ ! -e "$update_dir/${file#$script_dir/}" ]; then
                rm "$file"
            fi
        done

        # 调整权限
        chmod -R 755 "$script_dir"
        # 删除.git
        rm -rf "$script_dir/.git"

        # 清理临时文件
        rm -rf "$temp_dir"

        log_success "更新完成，新版本：${latest_version}"
        log_info "请重新运行脚本以使用新版本"
        exit 0
    fi
}

# 主函数
main() {
    initialize_config
    log_environment_info
    # 检查是否有 -y 参数
    if [[ "$*" == *"-y"* || "$*" == *"--yes"* ]]; then
        sudo -v || {
            log_error "无法获取 sudo 权限"
            exit 1
        }
        sudo_keep_alive
        trap cleanup EXIT
    fi
    # 检查是否是首次运行
    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${COLOR_YELLOW}欢迎使用 Fridare！这似乎是您第一次运行本工具。${COLOR_RESET}"
        echo -e "${COLOR_YELLOW}以下是快速开始指南：${COLOR_RESET}\n"
        quick_start_guide
        echo -e "\n${COLOR_YELLOW}按回车键继续...${COLOR_RESET}"
        read
    fi
    # 读取配置文件（如果存在）
    if [ -f "$CONFIG_FILE" ]; then
        FRIDA_MODULES=()
        source "$CONFIG_FILE"
    fi

    # 将配置文件中的值赋给脚本变量
    FRIDA_SERVER_PORT=${FRIDA_SERVER_PORT:-$DEF_FRIDA_SERVER_PORT}
    CURL_PROXY=${CURL_PROXY:-""}
    AUTO_CONFIRM=${AUTO_CONFIRM:-$DEF_AUTO_CONFIRM}
    FRIDA_NAME=${FRIDA_NAME:-""}

    log_config_info

    # 解析参数
    parse_arguments "$@"
}
# 执行主函数
main "$@"
