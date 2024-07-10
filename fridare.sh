#!/bin/bash

# Frida 魔改脚本，用于修改 frida-server 的名称和端口
# 作者：suifei@gmail.com

set -e # 遇到错误立即退出

VERSION="3.0.0"
# 默认值设置
# DEF_FRIDA_VERSION=
DEF_FRIDA_SERVER_PORT=8899
# DEF_HTTP_PROXY=""
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
CONFIG_FILE="${HOME}/.fridare_config"

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

show_main_usage() {
    echo -e "${COLOR_SKYBLUE}Frida 重打包工具 v${VERSION}${COLOR_RESET}"
    echo
    echo "用法: $0 <命令> [选项]"
    echo
    echo "命令:"
    echo "  build                 重新打包 Frida"
    echo "  ls, list              列出可用的 Frida 版本" # 完成, complete
    echo "  download              下载特定版本的 Frida"  # 完成, complete
    echo "  lm, list-modules      列出可用的 Frida 模块" # 完成, complete
    echo "  setup                 检查并安装系统依赖"      # 完成, complete
    echo "  config                设置配置选项"         # 完成, complete
    echo "  help                  显示帮助信息"         # 完成, complete
    echo
    echo "运行 '$0 help <命令>' 以获取特定命令的更多信息。"
    echo "    suifei@gmail.com"
    echo "    https://github.com/suifei/fridare"
}
show_build_usage() {
    echo "用法: $0 build [选项]"
    echo
    echo "选项:"
    echo "  -v VERSION               指定 Frida 版本"
    echo "  -latest                  使用最新的 Frida 版本"
    echo "  -p, --port PORT          指定 Frida 服务器端口 (默认: $DEF_FRIDA_SERVER_PORT)"
    echo "  -y, --yes                自动回答是以确认提示"
    echo
    echo "注意: -v 和 -latest 不能同时使用"
}
show_config_usage() {
    echo "用法: $0 config <操作> <选项> [<值>]"
    echo
    echo "操作:"
    echo "  set <选项> <值>    设置配置"
    echo "  unset <选项>       取消设置"
    echo "  ls, list          列出所有配置"
    echo
    echo "选项:"
    echo "  proxy              HTTP 代理"
    echo "  port               Frida 服务器端口"
    echo "  frida-name         Frida 魔改名"
}
show_download_usage() {
    echo "用法: $0 download [选项] <输出目录>"
    echo
    echo "选项:"
    echo "  -v, --version VERSION    指定要下载的 Frida 版本"
    echo "  -latest                  下载最新的 Frida 版本"
    echo "  -m, --module MODULE      指定要下载的模块名称"
    echo "  -all                     下载所有模块"
    echo "  --no-extract             不自动解压文件"
    echo "  lm, list-modules         列出所有可用的模块"
    echo
    echo "示例:"
    echo "  $0 download -v 16.4.2 -m frida-server ./output"
    echo "  $0 download -latest -m frida-gadget ./output"
    echo "  $0 download -latest -all ./output"
    echo "  $0 download -latest -all --no-extract ./output"
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
    build)
        parse_build_args "$@"
        ;;
    setup)
        setup_environment
        ;;
    config)
        parse_config_args "$@"
        ;;
    ls | list)
        list_frida_versions
        ;;
    download)
        parse_download_args "$@"
        ;;
    lm | list-modules)
        list_frida_modules
        ;;
    help)
        if [ $# -eq 0 ]; then
            show_main_usage
        else
            case "$1" in
            build) show_build_usage ;;
            config) show_config_usage ;;
            download) show_download_usage ;;
            *)
                log_error "未知命令: $1"
                show_main_usage
                ;;
            esac
        fi
        ;;
    *)
        log_error "未知命令: $command"
        show_main_usage
        exit 1
        ;;
    esac
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

    while [ "$#" -gt 0 ]; do
        case "$1" in
        -v)
            if [ "$USE_LATEST" = "true" ]; then
                log_error "错误: -v 和 -latest 不能同时使用" >&2
                show_build_usage
                exit 1
            fi
            FRIDA_VERSION="$2"
            shift 2
            ;;
        -latest)
            if [ -n "$FRIDA_VERSION" ]; then
                log_error "错误: -v 和 -latest 不能同时使用" >&2
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
    elif [ -z "$FRIDA_VERSION" ]; then
        log_error "错误: 必须指定 Frida 版本 (-v) 或使用最新版本 (-latest)" >&2
        show_build_usage
        exit 1
    else
        log_info "使用指定的 Frida 版本: $FRIDA_VERSION"
    fi

    # 执行构建逻辑
    build_frida
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
        ;;
    unset)
        if [ $# -lt 1 ]; then
            log_error "unset 命令需要一个选项"
            show_config_usage
            exit 1
        fi
        option="$1"
        unset_config "$option"
        ;;
    ls | list)
        list_config
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
        log_success "HTTP 代理已设置为: $CURL_PROXY"
        ;;
    port)
        FRIDA_SERVER_PORT="$value"
        log_success "Frida 服务器端口已设置为: $FRIDA_SERVER_PORT"
        ;;
    frida-name)
        if [[ "$value" =~ ^[a-zA-Z]{5}$ ]]; then
            FRIDA_NAME="$value"
            log_success "Frida 魔改名已设置为: $FRIDA_NAME"
        else
            log_error "无效的 Frida 魔改名: $value"
            log_info "Frida 魔改名必须是恰好 5 个字母（a-z 或 A-Z）"
            return 1
        fi
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
    # 读取现有配置
    local current_config=$(cat "$CONFIG_FILE")

    # 更新配置
    echo "# Fridare Configuration File" > "$CONFIG_FILE"
    echo "$current_config" | while IFS='=' read -r key value; do
        case "$key" in
            FRIDA_SERVER_PORT)
                echo "FRIDA_SERVER_PORT=${FRIDA_SERVER_PORT:-$value}"
                ;;
            CURL_PROXY)
                echo "CURL_PROXY=${CURL_PROXY:-$value}"
                ;;
            AUTO_CONFIRM)
                echo "AUTO_CONFIRM=${AUTO_CONFIRM:-$value}"
                ;;
            FRIDA_NAME)
                echo "FRIDA_NAME=${FRIDA_NAME:-$value}"
                ;;
            *)
                echo "$key=$value"
                ;;
        esac
    done >> "$CONFIG_FILE"

    log_success "配置已更新: $CONFIG_FILE"
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
    echo "----------------------------------------------------------------"

    echo "$releases" | jq -r '.[] | "\(.tag_name)\t\(.published_at)\t\(.assets[0].download_count)"' | 
    while IFS=$'\t' read -r version date downloads; do
        # 格式化日期 (适用于 macOS)
        formatted_date=$(date -jf "%Y-%m-%dT%H:%M:%SZ" "$date" "+%Y-%m-%d" 2>/dev/null || echo "$date")
        
        # 获取版本说明的前100个字符
        description=$(echo "$releases" | jq -r ".[] | select(.tag_name == \"$version\") | .body" | sed 's/\r//' | head -n 1 | cut -c 1-100)

        # 输出格式化的版本信息
        printf "${COLOR_GREEN}%2d${COLOR_RESET}\t${COLOR_YELLOW}%-10s${COLOR_RESET}\t%s\t%'d\n" "$((++i))" "$version" "$formatted_date" "$downloads"
        echo -e "${COLOR_PURPLE}   说明: ${COLOR_RESET}${description}...\n"
    done

    echo -e "\n${COLOR_SKYBLUE}提示: 使用 'fridare.sh build -v <版本号>' 来构建特定版本${COLOR_RESET}"
}
FRIDA_MODULES=(
    "frida-python-cp37-abi3:freebsd:amd64:frida-{VERSION}-cp37-abi3-freebsd_14_0_release_amd64.whl"
    "frida-clr:windows:x86:frida-clr-{VERSION}-windows-x86.dll.xz"
    "frida-clr:windows:x86_64:frida-clr-{VERSION}-windows-x86_64.dll.xz"
    "frida-core-devkit:android:arm:frida-core-devkit-{VERSION}-android-arm.tar.xz"
    "frida-core-devkit:android:arm64:frida-core-devkit-{VERSION}-android-arm64.tar.xz"
    "frida-core-devkit:android:x86:frida-core-devkit-{VERSION}-android-x86.tar.xz"
    "frida-core-devkit:android:x86_64:frida-core-devkit-{VERSION}-android-x86_64.tar.xz"
    "frida-core-devkit:freebsd:arm64:frida-core-devkit-{VERSION}-freebsd-arm64.tar.xz"
    "frida-core-devkit:freebsd:x86_64:frida-core-devkit-{VERSION}-freebsd-x86_64.tar.xz"
    "frida-core-devkit:ios:simulator:frida-core-devkit-{VERSION}-ios-arm64-simulator.tar.xz"
    "frida-core-devkit:ios:arm64:frida-core-devkit-{VERSION}-ios-arm64.tar.xz"
    "frida-core-devkit:ios:arm64e:frida-core-devkit-{VERSION}-ios-arm64e.tar.xz"
    "frida-core-devkit:ios:x86_64:frida-core-devkit-{VERSION}-ios-x86_64-simulator.tar.xz"
    "frida-core-devkit:linux:musl:frida-core-devkit-{VERSION}-linux-arm64-musl.tar.xz"
    "frida-core-devkit:linux:arm64:frida-core-devkit-{VERSION}-linux-arm64.tar.xz"
    "frida-core-devkit:linux:armhf:frida-core-devkit-{VERSION}-linux-armhf.tar.xz"
    "frida-core-devkit:linux:mips:frida-core-devkit-{VERSION}-linux-mips.tar.xz"
    "frida-core-devkit:linux:mips64:frida-core-devkit-{VERSION}-linux-mips64.tar.xz"
    "frida-core-devkit:linux:mips64el:frida-core-devkit-{VERSION}-linux-mips64el.tar.xz"
    "frida-core-devkit:linux:mipsel:frida-core-devkit-{VERSION}-linux-mipsel.tar.xz"
    "frida-core-devkit:linux:x86:frida-core-devkit-{VERSION}-linux-x86.tar.xz"
    "frida-core-devkit:linux:x86_64:frida-core-devkit-{VERSION}-linux-x86_64-musl.tar.xz"
    "frida-core-devkit:linux:x86_64:frida-core-devkit-{VERSION}-linux-x86_64.tar.xz"
    "frida-core-devkit:macos:arm64:frida-core-devkit-{VERSION}-macos-arm64.tar.xz"
    "frida-core-devkit:macos:arm64e:frida-core-devkit-{VERSION}-macos-arm64e.tar.xz"
    "frida-core-devkit:macos:x86_64:frida-core-devkit-{VERSION}-macos-x86_64.tar.xz"
    "frida-core-devkit:qnx:armeabi:frida-core-devkit-{VERSION}-qnx-armeabi.tar.xz"
    "frida-core-devkit:tvos:simulator:frida-core-devkit-{VERSION}-tvos-arm64-simulator.tar.xz"
    "frida-core-devkit:tvos:arm64:frida-core-devkit-{VERSION}-tvos-arm64.tar.xz"
    "frida-core-devkit:watchos:simulator:frida-core-devkit-{VERSION}-watchos-arm64-simulator.tar.xz"
    "frida-core-devkit:watchos:arm64:frida-core-devkit-{VERSION}-watchos-arm64.tar.xz"
    "frida-core-devkit:windows:x86:frida-core-devkit-{VERSION}-windows-x86.exe"
    "frida-core-devkit:windows:x86:frida-core-devkit-{VERSION}-windows-x86.tar.xz"
    "frida-core-devkit:windows:x86_64:frida-core-devkit-{VERSION}-windows-x86_64.exe"
    "frida-core-devkit:windows:x86_64:frida-core-devkit-{VERSION}-windows-x86_64.tar.xz"
    "frida-gadget:android:arm:frida-gadget-{VERSION}-android-arm.so.xz"
    "frida-gadget:android:arm64:frida-gadget-{VERSION}-android-arm64.so.xz"
    "frida-gadget:android:x86:frida-gadget-{VERSION}-android-x86.so.xz"
    "frida-gadget:android:x86_64:frida-gadget-{VERSION}-android-x86_64.so.xz"
    "frida-gadget:freebsd:arm64:frida-gadget-{VERSION}-freebsd-arm64.so.xz"
    "frida-gadget:freebsd:x86_64:frida-gadget-{VERSION}-freebsd-x86_64.so.xz"
    "frida-gadget:ios:universal:frida-gadget-{VERSION}-ios-simulator-universal.dylib.xz"
    "frida-gadget:ios:universal:frida-gadget-{VERSION}-ios-universal.dylib.gz"
    "frida-gadget:ios:universal:frida-gadget-{VERSION}-ios-universal.dylib.xz"
    "frida-gadget:linux:musl:frida-gadget-{VERSION}-linux-arm64-musl.so.xz"
    "frida-gadget:linux:arm64:frida-gadget-{VERSION}-linux-arm64.so.xz"
    "frida-gadget:linux:armhf:frida-gadget-{VERSION}-linux-armhf.so.xz"
    "frida-gadget:linux:mips:frida-gadget-{VERSION}-linux-mips.so.xz"
    "frida-gadget:linux:mips64:frida-gadget-{VERSION}-linux-mips64.so.xz"
    "frida-gadget:linux:mips64el:frida-gadget-{VERSION}-linux-mips64el.so.xz"
    "frida-gadget:linux:mipsel:frida-gadget-{VERSION}-linux-mipsel.so.xz"
    "frida-gadget:linux:x86:frida-gadget-{VERSION}-linux-x86.so.xz"
    "frida-gadget:linux:x86_64:frida-gadget-{VERSION}-linux-x86_64-musl.so.xz"
    "frida-gadget:linux:x86_64:frida-gadget-{VERSION}-linux-x86_64.so.xz"
    "frida-gadget:macos:universal:frida-gadget-{VERSION}-macos-universal.dylib.xz"
    "frida-gadget:qnx:armeabi:frida-gadget-{VERSION}-qnx-armeabi.so.xz"
    "frida-gadget:tvos:simulator:frida-gadget-{VERSION}-tvos-arm64-simulator.dylib.xz"
    "frida-gadget:tvos:arm64:frida-gadget-{VERSION}-tvos-arm64.dylib.xz"
    "frida-gadget:watchos:simulator:frida-gadget-{VERSION}-watchos-arm64-simulator.dylib.xz"
    "frida-gadget:watchos:arm64:frida-gadget-{VERSION}-watchos-arm64.dylib.xz"
    "frida-gadget:windows:x86:frida-gadget-{VERSION}-windows-x86.dll.xz"
    "frida-gadget:windows:x86_64:frida-gadget-{VERSION}-windows-x86_64.dll.xz"
    "frida-gum-devkit:android:arm:frida-gum-devkit-{VERSION}-android-arm.tar.xz"
    "frida-gum-devkit:android:arm64:frida-gum-devkit-{VERSION}-android-arm64.tar.xz"
    "frida-gum-devkit:android:x86:frida-gum-devkit-{VERSION}-android-x86.tar.xz"
    "frida-gum-devkit:android:x86_64:frida-gum-devkit-{VERSION}-android-x86_64.tar.xz"
    "frida-gum-devkit:freebsd:arm64:frida-gum-devkit-{VERSION}-freebsd-arm64.tar.xz"
    "frida-gum-devkit:freebsd:x86_64:frida-gum-devkit-{VERSION}-freebsd-x86_64.tar.xz"
    "frida-gum-devkit:ios:simulator:frida-gum-devkit-{VERSION}-ios-arm64-simulator.tar.xz"
    "frida-gum-devkit:ios:arm64:frida-gum-devkit-{VERSION}-ios-arm64.tar.xz"
    "frida-gum-devkit:ios:arm64e:frida-gum-devkit-{VERSION}-ios-arm64e.tar.xz"
    "frida-gum-devkit:ios:x86_64:frida-gum-devkit-{VERSION}-ios-x86_64-simulator.tar.xz"
    "frida-gum-devkit:linux:musl:frida-gum-devkit-{VERSION}-linux-arm64-musl.tar.xz"
    "frida-gum-devkit:linux:arm64:frida-gum-devkit-{VERSION}-linux-arm64.tar.xz"
    "frida-gum-devkit:linux:armhf:frida-gum-devkit-{VERSION}-linux-armhf.tar.xz"
    "frida-gum-devkit:linux:mips:frida-gum-devkit-{VERSION}-linux-mips.tar.xz"
    "frida-gum-devkit:linux:mips64:frida-gum-devkit-{VERSION}-linux-mips64.tar.xz"
    "frida-gum-devkit:linux:mips64el:frida-gum-devkit-{VERSION}-linux-mips64el.tar.xz"
    "frida-gum-devkit:linux:mipsel:frida-gum-devkit-{VERSION}-linux-mipsel.tar.xz"
    "frida-gum-devkit:linux:x86:frida-gum-devkit-{VERSION}-linux-x86.tar.xz"
    "frida-gum-devkit:linux:x86_64:frida-gum-devkit-{VERSION}-linux-x86_64-musl.tar.xz"
    "frida-gum-devkit:linux:x86_64:frida-gum-devkit-{VERSION}-linux-x86_64.tar.xz"
    "frida-gum-devkit:macos:arm64:frida-gum-devkit-{VERSION}-macos-arm64.tar.xz"
    "frida-gum-devkit:macos:arm64e:frida-gum-devkit-{VERSION}-macos-arm64e.tar.xz"
    "frida-gum-devkit:macos:x86_64:frida-gum-devkit-{VERSION}-macos-x86_64.tar.xz"
    "frida-gum-devkit:qnx:armeabi:frida-gum-devkit-{VERSION}-qnx-armeabi.tar.xz"
    "frida-gum-devkit:tvos:simulator:frida-gum-devkit-{VERSION}-tvos-arm64-simulator.tar.xz"
    "frida-gum-devkit:tvos:arm64:frida-gum-devkit-{VERSION}-tvos-arm64.tar.xz"
    "frida-gum-devkit:watchos:simulator:frida-gum-devkit-{VERSION}-watchos-arm64-simulator.tar.xz"
    "frida-gum-devkit:watchos:arm64:frida-gum-devkit-{VERSION}-watchos-arm64.tar.xz"
    "frida-gum-devkit:windows:x86:frida-gum-devkit-{VERSION}-windows-x86.exe"
    "frida-gum-devkit:windows:x86:frida-gum-devkit-{VERSION}-windows-x86.tar.xz"
    "frida-gum-devkit:windows:x86_64:frida-gum-devkit-{VERSION}-windows-x86_64.exe"
    "frida-gum-devkit:windows:x86_64:frida-gum-devkit-{VERSION}-windows-x86_64.tar.xz"
    "frida-gumjs-devkit:android:arm:frida-gumjs-devkit-{VERSION}-android-arm.tar.xz"
    "frida-gumjs-devkit:android:arm64:frida-gumjs-devkit-{VERSION}-android-arm64.tar.xz"
    "frida-gumjs-devkit:android:x86:frida-gumjs-devkit-{VERSION}-android-x86.tar.xz"
    "frida-gumjs-devkit:android:x86_64:frida-gumjs-devkit-{VERSION}-android-x86_64.tar.xz"
    "frida-gumjs-devkit:freebsd:arm64:frida-gumjs-devkit-{VERSION}-freebsd-arm64.tar.xz"
    "frida-gumjs-devkit:freebsd:x86_64:frida-gumjs-devkit-{VERSION}-freebsd-x86_64.tar.xz"
    "frida-gumjs-devkit:ios:simulator:frida-gumjs-devkit-{VERSION}-ios-arm64-simulator.tar.xz"
    "frida-gumjs-devkit:ios:arm64:frida-gumjs-devkit-{VERSION}-ios-arm64.tar.xz"
    "frida-gumjs-devkit:ios:arm64e:frida-gumjs-devkit-{VERSION}-ios-arm64e.tar.xz"
    "frida-gumjs-devkit:ios:x86_64:frida-gumjs-devkit-{VERSION}-ios-x86_64-simulator.tar.xz"
    "frida-gumjs-devkit:linux:musl:frida-gumjs-devkit-{VERSION}-linux-arm64-musl.tar.xz"
    "frida-gumjs-devkit:linux:arm64:frida-gumjs-devkit-{VERSION}-linux-arm64.tar.xz"
    "frida-gumjs-devkit:linux:armhf:frida-gumjs-devkit-{VERSION}-linux-armhf.tar.xz"
    "frida-gumjs-devkit:linux:mips:frida-gumjs-devkit-{VERSION}-linux-mips.tar.xz"
    "frida-gumjs-devkit:linux:mips64:frida-gumjs-devkit-{VERSION}-linux-mips64.tar.xz"
    "frida-gumjs-devkit:linux:mips64el:frida-gumjs-devkit-{VERSION}-linux-mips64el.tar.xz"
    "frida-gumjs-devkit:linux:mipsel:frida-gumjs-devkit-{VERSION}-linux-mipsel.tar.xz"
    "frida-gumjs-devkit:linux:x86:frida-gumjs-devkit-{VERSION}-linux-x86.tar.xz"
    "frida-gumjs-devkit:linux:x86_64:frida-gumjs-devkit-{VERSION}-linux-x86_64-musl.tar.xz"
    "frida-gumjs-devkit:linux:x86_64:frida-gumjs-devkit-{VERSION}-linux-x86_64.tar.xz"
    "frida-gumjs-devkit:macos:arm64:frida-gumjs-devkit-{VERSION}-macos-arm64.tar.xz"
    "frida-gumjs-devkit:macos:arm64e:frida-gumjs-devkit-{VERSION}-macos-arm64e.tar.xz"
    "frida-gumjs-devkit:macos:x86_64:frida-gumjs-devkit-{VERSION}-macos-x86_64.tar.xz"
    "frida-gumjs-devkit:qnx:armeabi:frida-gumjs-devkit-{VERSION}-qnx-armeabi.tar.xz"
    "frida-gumjs-devkit:tvos:simulator:frida-gumjs-devkit-{VERSION}-tvos-arm64-simulator.tar.xz"
    "frida-gumjs-devkit:tvos:arm64:frida-gumjs-devkit-{VERSION}-tvos-arm64.tar.xz"
    "frida-gumjs-devkit:watchos:simulator:frida-gumjs-devkit-{VERSION}-watchos-arm64-simulator.tar.xz"
    "frida-gumjs-devkit:watchos:arm64:frida-gumjs-devkit-{VERSION}-watchos-arm64.tar.xz"
    "frida-gumjs-devkit:windows:x86:frida-gumjs-devkit-{VERSION}-windows-x86.exe"
    "frida-gumjs-devkit:windows:x86:frida-gumjs-devkit-{VERSION}-windows-x86.tar.xz"
    "frida-gumjs-devkit:windows:x86_64:frida-gumjs-devkit-{VERSION}-windows-x86_64.exe"
    "frida-gumjs-devkit:windows:x86_64:frida-gumjs-devkit-{VERSION}-windows-x86_64.tar.xz"
    "frida-inject:android:arm:frida-inject-{VERSION}-android-arm.xz"
    "frida-inject:android:arm64:frida-inject-{VERSION}-android-arm64.xz"
    "frida-inject:android:x86:frida-inject-{VERSION}-android-x86.xz"
    "frida-inject:android:x86_64:frida-inject-{VERSION}-android-x86_64.xz"
    "frida-inject:freebsd:arm64:frida-inject-{VERSION}-freebsd-arm64.xz"
    "frida-inject:freebsd:x86_64:frida-inject-{VERSION}-freebsd-x86_64.xz"
    "frida-inject:linux:musl:frida-inject-{VERSION}-linux-arm64-musl.xz"
    "frida-inject:linux:arm64:frida-inject-{VERSION}-linux-arm64.xz"
    "frida-inject:linux:armhf:frida-inject-{VERSION}-linux-armhf.xz"
    "frida-inject:linux:mips:frida-inject-{VERSION}-linux-mips.xz"
    "frida-inject:linux:mips64:frida-inject-{VERSION}-linux-mips64.xz"
    "frida-inject:linux:mips64el:frida-inject-{VERSION}-linux-mips64el.xz"
    "frida-inject:linux:mipsel:frida-inject-{VERSION}-linux-mipsel.xz"
    "frida-inject:linux:x86:frida-inject-{VERSION}-linux-x86.xz"
    "frida-inject:linux:x86_64:frida-inject-{VERSION}-linux-x86_64-musl.xz"
    "frida-inject:linux:x86_64:frida-inject-{VERSION}-linux-x86_64.xz"
    "frida-inject:macos:arm64:frida-inject-{VERSION}-macos-arm64.xz"
    "frida-inject:macos:arm64e:frida-inject-{VERSION}-macos-arm64e.xz"
    "frida-inject:macos:x86_64:frida-inject-{VERSION}-macos-x86_64.xz"
    "frida-inject:qnx:armeabi:frida-inject-{VERSION}-qnx-armeabi.xz"
    "frida-inject:windows:x86:frida-inject-{VERSION}-windows-x86.exe.xz"
    "frida-inject:windows:x86_64:frida-inject-{VERSION}-windows-x86_64.exe.xz"
    "frida-portal:android:arm:frida-portal-{VERSION}-android-arm.xz"
    "frida-portal:android:arm64:frida-portal-{VERSION}-android-arm64.xz"
    "frida-portal:android:x86:frida-portal-{VERSION}-android-x86.xz"
    "frida-portal:android:x86_64:frida-portal-{VERSION}-android-x86_64.xz"
    "frida-portal:freebsd:arm64:frida-portal-{VERSION}-freebsd-arm64.xz"
    "frida-portal:freebsd:x86_64:frida-portal-{VERSION}-freebsd-x86_64.xz"
    "frida-portal:ios:arm64:frida-portal-{VERSION}-ios-arm64.xz"
    "frida-portal:ios:arm64e:frida-portal-{VERSION}-ios-arm64e.xz"
    "frida-portal:linux:musl:frida-portal-{VERSION}-linux-arm64-musl.xz"
    "frida-portal:linux:arm64:frida-portal-{VERSION}-linux-arm64.xz"
    "frida-portal:linux:armhf:frida-portal-{VERSION}-linux-armhf.xz"
    "frida-portal:linux:mips:frida-portal-{VERSION}-linux-mips.xz"
    "frida-portal:linux:mips64:frida-portal-{VERSION}-linux-mips64.xz"
    "frida-portal:linux:mips64el:frida-portal-{VERSION}-linux-mips64el.xz"
    "frida-portal:linux:mipsel:frida-portal-{VERSION}-linux-mipsel.xz"
    "frida-portal:linux:x86:frida-portal-{VERSION}-linux-x86.xz"
    "frida-portal:linux:x86_64:frida-portal-{VERSION}-linux-x86_64-musl.xz"
    "frida-portal:linux:x86_64:frida-portal-{VERSION}-linux-x86_64.xz"
    "frida-portal:macos:arm64:frida-portal-{VERSION}-macos-arm64.xz"
    "frida-portal:macos:arm64e:frida-portal-{VERSION}-macos-arm64e.xz"
    "frida-portal:macos:x86_64:frida-portal-{VERSION}-macos-x86_64.xz"
    "frida-portal:qnx:armeabi:frida-portal-{VERSION}-qnx-armeabi.xz"
    "frida-portal:windows:x86:frida-portal-{VERSION}-windows-x86.exe.xz"
    "frida-portal:windows:x86_64:frida-portal-{VERSION}-windows-x86_64.exe.xz"
    "frida-qml:linux:x86_64:frida-qml-{VERSION}-linux-x86_64.tar.xz"
    "frida-qml:macos:x86_64:frida-qml-{VERSION}-macos-x86_64.tar.xz"
    "frida-qml:windows:x86_64:frida-qml-{VERSION}-windows-x86_64.tar.xz"
    "frida-server:android:arm:frida-server-{VERSION}-android-arm.xz"
    "frida-server:android:arm64:frida-server-{VERSION}-android-arm64.xz"
    "frida-server:android:x86:frida-server-{VERSION}-android-x86.xz"
    "frida-server:android:x86_64:frida-server-{VERSION}-android-x86_64.xz"
    "frida-server:freebsd:arm64:frida-server-{VERSION}-freebsd-arm64.xz"
    "frida-server:freebsd:x86_64:frida-server-{VERSION}-freebsd-x86_64.xz"
    "frida-server:linux:musl:frida-server-{VERSION}-linux-arm64-musl.xz"
    "frida-server:linux:arm64:frida-server-{VERSION}-linux-arm64.xz"
    "frida-server:linux:armhf:frida-server-{VERSION}-linux-armhf.xz"
    "frida-server:linux:mips:frida-server-{VERSION}-linux-mips.xz"
    "frida-server:linux:mips64:frida-server-{VERSION}-linux-mips64.xz"
    "frida-server:linux:mips64el:frida-server-{VERSION}-linux-mips64el.xz"
    "frida-server:linux:mipsel:frida-server-{VERSION}-linux-mipsel.xz"
    "frida-server:linux:x86:frida-server-{VERSION}-linux-x86.xz"
    "frida-server:linux:x86_64:frida-server-{VERSION}-linux-x86_64-musl.xz"
    "frida-server:linux:x86_64:frida-server-{VERSION}-linux-x86_64.xz"
    "frida-server:macos:arm64:frida-server-{VERSION}-macos-arm64.xz"
    "frida-server:macos:arm64e:frida-server-{VERSION}-macos-arm64e.xz"
    "frida-server:macos:x86_64:frida-server-{VERSION}-macos-x86_64.xz"
    "frida-server:qnx:armeabi:frida-server-{VERSION}-qnx-armeabi.xz"
    "frida-server:windows:x86:frida-server-{VERSION}-windows-x86.exe.xz"
    "frida-server:windows:x86_64:frida-server-{VERSION}-windows-x86_64.exe.xz"
    "frida-electron-v123:freebsd:arm64:frida-v{VERSION}-electron-v123-freebsd-arm64.tar.gz"
    "frida-electron-v123:freebsd:x64:frida-v{VERSION}-electron-v123-freebsd-x64.tar.gz"
    "frida-electron-v125:darwin:arm64:frida-v{VERSION}-electron-v125-darwin-arm64.tar.gz"
    "frida-electron-v125:darwin:x64:frida-v{VERSION}-electron-v125-darwin-x64.tar.gz"
    "frida-electron-v125:linux:arm64:frida-v{VERSION}-electron-v125-linux-arm64.tar.gz"
    "frida-electron-v125:linux:x64:frida-v{VERSION}-electron-v125-linux-x64.tar.gz"
    "frida-electron-v125:win32:x64:frida-v{VERSION}-electron-v125-win32-x64.tar.gz"
    "frida-node-v108:darwin:arm64:frida-v{VERSION}-node-v108-darwin-arm64.tar.gz"
    "frida-node-v108:darwin:x64:frida-v{VERSION}-node-v108-darwin-x64.tar.gz"
    "frida-node-v108:linux:arm64:frida-v{VERSION}-node-v108-linux-arm64.tar.gz"
    "frida-node-v108:linux:armv7l:frida-v{VERSION}-node-v108-linux-armv7l.tar.gz"
    "frida-node-v108:linux:ia32:frida-v{VERSION}-node-v108-linux-ia32.tar.gz"
    "frida-node-v108:linux:x64:frida-v{VERSION}-node-v108-linux-x64.tar.gz"
    "frida-node-v108:win32:x64:frida-v{VERSION}-node-v108-win32-x64.tar.gz"
    "frida-node-v115:darwin:arm64:frida-v{VERSION}-node-v115-darwin-arm64.tar.gz"
    "frida-node-v115:darwin:x64:frida-v{VERSION}-node-v115-darwin-x64.tar.gz"
    "frida-node-v115:freebsd:arm64:frida-v{VERSION}-node-v115-freebsd-arm64.tar.gz"
    "frida-node-v115:freebsd:x64:frida-v{VERSION}-node-v115-freebsd-x64.tar.gz"
    "frida-node-v115:linux:arm64:frida-v{VERSION}-node-v115-linux-arm64.tar.gz"
    "frida-node-v115:linux:armv7l:frida-v{VERSION}-node-v115-linux-armv7l.tar.gz"
    "frida-node-v115:linux:ia32:frida-v{VERSION}-node-v115-linux-ia32.tar.gz"
    "frida-node-v115:linux:x64:frida-v{VERSION}-node-v115-linux-x64.tar.gz"
    "frida-node-v115:win32:x64:frida-v{VERSION}-node-v115-win32-x64.tar.gz"
    "frida-node-v127:darwin:arm64:frida-v{VERSION}-node-v127-darwin-arm64.tar.gz"
    "frida-node-v127:darwin:x64:frida-v{VERSION}-node-v127-darwin-x64.tar.gz"
    "frida-node-v127:linux:arm64:frida-v{VERSION}-node-v127-linux-arm64.tar.gz"
    "frida-node-v127:linux:armv7l:frida-v{VERSION}-node-v127-linux-armv7l.tar.gz"
    "frida-node-v127:linux:ia32:frida-v{VERSION}-node-v127-linux-ia32.tar.gz"
    "frida-node-v127:linux:x64:frida-v{VERSION}-node-v127-linux-x64.tar.gz"
    "frida-node-v127:win32:x64:frida-v{VERSION}-node-v127-win32-x64.tar.gz"
    "frida-node-v93:darwin:arm64:frida-v{VERSION}-node-v93-darwin-arm64.tar.gz"
    "frida-node-v93:darwin:x64:frida-v{VERSION}-node-v93-darwin-x64.tar.gz"
    "frida-node-v93:linux:arm64:frida-v{VERSION}-node-v93-linux-arm64.tar.gz"
    "frida-node-v93:linux:armv7l:frida-v{VERSION}-node-v93-linux-armv7l.tar.gz"
    "frida-node-v93:linux:ia32:frida-v{VERSION}-node-v93-linux-ia32.tar.gz"
    "frida-node-v93:linux:x64:frida-v{VERSION}-node-v93-linux-x64.tar.gz"
    "frida-node-v93:win32:ia32:frida-v{VERSION}-node-v93-win32-ia32.tar.gz"
    "frida-node-v93:win32:x64:frida-v{VERSION}-node-v93-win32-x64.tar.gz"
    "frida-appletvos-deb:appletvos:arm64:frida_{VERSION}_appletvos-arm64.deb"
    "frida-iphoneos-deb:iphoneos:arm:frida_{VERSION}_iphoneos-arm.deb"
    "frida-iphoneos-deb:iphoneos:arm64:frida_{VERSION}_iphoneos-arm64.deb"
    "gum-graft:macos:arm64:gum-graft-{VERSION}-macos-arm64.xz"
    "gum-graft:macos:x86_64:gum-graft-{VERSION}-macos-x86_64.xz"
)
list_frida_modules() {
    log_info "可用的 Frida 模块："
    echo -e "${COLOR_GREEN}模块名称\t\t操作系统\t架构${COLOR_RESET}"
    echo "----------------------------------------"

    # 使用临时文件来存储和排序唯一的模块
    temp_file=$(mktemp)

    for item in "${FRIDA_MODULES[@]}"; do
        IFS=':' read -r mod os arch filename <<<"$item"
        echo "$mod	$os	$arch" >>"$temp_file"
    done

    # 排序并移除重复项
    sort -u "$temp_file" | while read -r line; do
        echo -e "$line"
    done

    # 删除临时文件
    rm -f "$temp_file"
}
parse_download_args() {
    local version=""
    local module=""
    local output_dir=""
    local use_latest=false
    local download_all=false
    local no_extract=false

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
    download_frida_module "$version" "$use_latest" "$module" "$download_all" "$output_dir" "$no_extract"
}
download_frida_module() {
    local version="$1"
    local use_latest="$2"
    local module="$3"
    local download_all="$4"
    local output_dir="$5"
    local no_extract="$6"

    # 如果使用最新版本，获取最新版本号
    if [[ "$use_latest" == true ]]; then
        version=$(get_latest_frida_version)
        log_info "使用最新版本: $version"
    fi
    # 如果指定了模块，检查模块是否存在
    if [[ -n "$module" ]]; then
        local module_found=false
        for item in "${FRIDA_MODULES[@]}"; do
            IFS=':' read -r mod os arch filename <<<"$item"
            if [[ "$mod" == "$module" ]]; then
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

    # 遍历模块列表
    for item in "${FRIDA_MODULES[@]}"; do
        IFS=':' read -r mod os arch filename <<<"$item"

        # 如果指定了模块且不匹配，或者不是下载全部，则跳过
        if [[ -n "$module" && "$mod" != "$module" ]] || [[ "$download_all" != true && -n "$module" && "$mod" != "$module" ]]; then
            continue
        fi

        # 替换文件名中的版本占位符
        filename="${filename/\{VERSION\}/$version}"

        # 创建目录结构
        local dir="${output_dir}/${version}/${mod}/${os}/${arch}"
        mkdir -p "$dir"

        local url="https://github.com/frida/frida/releases/download/${version}/${filename}"
        local output_file="${dir}/${filename}"

        log_info "正在下载 $filename 到 $dir"
        if [ -n "$CURL_PROXY" ]; then
            curl -L -o "$output_file" --proxy "$CURL_PROXY" "$url" || {
                log_error "下载 $filename 失败"
                continue
            }
        else
            curl -L -o "$output_file" "$url" || {
                log_error "下载 $filename 失败"
                continue
            }
        fi
        log_success "下载 $filename 完成"
        # 解压逻辑
        if [[ "$no_extract" != true ]]; then
            if [[ "$filename" != *.deb && "$filename" != *.whl ]]; then # 排除 deb 和 whl 文件
                if command -v 7z &>/dev/null; then
                    log_info "使用 7z 解压 $filename..."
                    7z x "$output_file" -o"$dir" -y || {
                        log_error "解压 $filename 失败"
                        continue
                    }
                else
                    case "$filename" in
                    *.tar.xz)
                        log_info "解压 $filename..."
                        tar -xJf "$output_file" -C "$dir" || {
                            log_error "解压 $filename 失败"
                            continue
                        }
                        ;;
                    *.xz)
                        log_info "解压 $filename..."
                        xz -d "$output_file" || {
                            log_error "解压 $filename 失败"
                            continue
                        }
                        ;;
                    *.gz)
                        log_info "解压 $filename..."
                        gzip -d "$output_file" || {
                            log_error "解压 $filename 失败"
                            continue
                        }
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

    log_success "所有下载和解压操作完成"
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

    log_success "依赖安装完成"
}

setup_environment() {
    log_info "检查系统依赖..."
    if ! check_dependencies; then
        install_dependencies
    fi

    # 检查并创建 build 和 dist 目录
    if [ ! -d "build" ]; then
        mkdir -p build
    fi
    if [ ! -d "dist" ]; then
        mkdir -p dist
    fi

    log_success "环境设置完成"
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
    local filename="frida_${FRIDA_VERSION}_iphoneos-${arch}.deb"
    if [ ! -f "$filename" ]; then
        log_info "下载 $filename"
        # 如果配置了代理 CURL_PROXY
        if [ -n "$CURL_PROXY" ]; then
            curl -L -o "$filename" --proxy "$CURL_PROXY" "https://github.com/frida/frida/releases/download/${FRIDA_VERSION}/${filename}" || {
                log_error "下载 $filename 失败"
                exit 1
            }
        else
            curl -L -o "$filename" "https://github.com/frida/frida/releases/download/${FRIDA_VERSION}/${filename}" || {
                log_error "下载 $filename 失败"
                exit 1
            }
        fi
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
    cd ../hexreplace
    go build -o ../build/hexreplace
    cd ../build
    chmod +x hexreplace
    ./hexreplace $frida_server_path $FRIDA_NAME $new_path
    rm -rf $frida_server_path
    # 确保新文件有执行权限
    sudo chmod +x $new_path
    sudo chown root:wheel $new_path

    ./hexreplace $frida_dylib_file $FRIDA_NAME $new_dylib_file
    rm -rf $frida_dylib_file
    # 确保新文件有执行权限
    sudo chmod +x $new_dylib_file
    sudo chown root:wheel $new_dylib_file

    # 修改dylib目录
    mv $dylib_folder $new_dylib_folder

    log_success "二进制文件修改完成"
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

# 函数：修订frida-tools
modify_frida_tools() {
    PYLIB_PATH=$(python3 -c "import os, frida; print(os.path.dirname(frida.__file__))")
    PYLIB=$(ls $PYLIB_PATH/*.so)

    if [ -z "$PYLIB" ]; then
        log_error "未找到 frida python 库"
        return 1
    fi

    # 判断是否有备份文件，没有则备份
    if [ ! -f "$PYLIB.fridare" ]; then
        cp "$PYLIB" "$PYLIB.fridare"
    fi

    log_info "python libfile: $PYLIB $FRIDA_NAME"

    ./hexreplace "$PYLIB" $FRIDA_NAME test.so

    rm -rf "$PYLIB" "$PYLIB_PATH/__pycache__"
    mv test.so "$PYLIB"
    chmod 755 "$PYLIB"

    python3 -c "
import os, frida, shutil, re;
p = os.path.join(os.path.dirname(frida.__file__), 'core.py');
b = p + '.fridare';
frida_name = '$FRIDA_NAME';
if not os.path.exists(b):
    print(f'Creating backup: {b}');
    shutil.copy2(p, b);
else:
    print(f'Backup already exists: {b}');
try:
    with open(p, 'r') as f:
        lines = f.readlines();
    replaced = False;
    for i, line in enumerate(lines):
        matches = re.finditer(r'\"([^\"]{5}):rpc\"', line)
        for match in matches:
            old = match.group(1)
            new = frida_name[:5].ljust(5)  # Ensure frida_name is 5 chars
            line = line.replace(f'\"{old}:rpc\"', f'\"{new}:rpc\"')
            print(f'Line {i+1}: Replaced \"{old}:rpc\" with \"{new}:rpc\"')
            replaced = True
        lines[i] = line
    if replaced:
        with open(p, 'w') as f:
            f.writelines(lines);
        print(f'Replacement complete');
    else:
        print('No matching pattern found, no changes made');
except Exception as e:
    print(f'Error: {e}');
    if os.path.exists(b):
        print('Restoring from backup');
        shutil.copy2(b, p);
    else:
        print('No backup found to restore from');
"
    log_success "frida-tools 修改完成"
}

build_frida() {
    # 检查并设置 Frida 版本
    if [ -z "$FRIDA_VERSION" ]; then
        log_error "未指定 Frida 版本"
        show_build_usage
        exit 1
    fi

    log_info "使用 Frida 服务器端口: $FRIDA_SERVER_PORT"
    [ -n "$CURL_PROXY" ] && log_info "HTTP 代理：${CURL_PROXY}"
    [ "$AUTO_CONFIRM" = "true" ] && log_info "自动确认：已启用"

    log_warning "期间可能会要求输入 sudo 密码，用于修改文件权限"

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
    cd build
    # 如果 FRIDA_NAME 为空，生成一个新的
    if [ -z "$FRIDA_NAME" ]; then
        FRIDA_NAME=$(cat /dev/urandom | env LC_CTYPE=C tr -dc 'a-z' | fold -w 5 | grep -E '^[a-z]+$' | head -n 1)
        if [[ ! "$FRIDA_NAME" =~ ^[a-z]{5}$ ]]; then
            log_error "无法生成有效的 Frida 魔改名"
            exit 1
        fi
    fi

    for arch in arm arm64; do
        download_frida $arch

        BUILD_DIR="frida_${FRIDA_VERSION}_iphoneos-${arch}"
        rm -rf "$BUILD_DIR"
        dpkg-deb -R "frida_${FRIDA_VERSION}_iphoneos-${arch}.deb" "$BUILD_DIR"

        log_cinfo $COLOR_GREEN "正在修改 Frida ${COLOR_PURPLE}${FRIDA_VERSION}${COLOR_RESET} 版本 (${COLOR_SKYBLUE}${arch}${COLOR_RESET})"
        modify_launch_daemon "$BUILD_DIR" "$arch"
        modify_debian_files "$BUILD_DIR" "$arch"
        modify_binary "$BUILD_DIR" "$arch"

        OUTPUT_FILENAME="frida_${FRIDA_VERSION}_iphoneos-${arch}_${FRIDA_NAME}_tcp.deb"
        repackage_deb "$BUILD_DIR" "$OUTPUT_FILENAME"

        mkdir -p ../dist
        mv "$OUTPUT_FILENAME" ../dist/
        log_success "Frida ${FRIDA_VERSION} 版本 (${arch}) 修改完成"
        log_info "新版本名：${FRIDA_NAME}"
        log_info "请使用新版本名：${FRIDA_NAME} 进行调试"
        log_info "请使用端口：${FRIDA_SERVER_PORT} 进行调试"
        log_info "新版本 deb 文件：../dist/${OUTPUT_FILENAME}"
        log_info "-------------------------------------------------"
        log_info "iPhone 安装："
        log_info "scp dist/${OUTPUT_FILENAME} root@<iPhone-IP>:/var/root"
        log_info "ssh root@<iPhone-IP>"
        log_info "dpkg -i /var/root/${OUTPUT_FILENAME}"
        log_info "PC 连接："
        log_info "frida -U -f com.xxx.xxx -l"
        log_info "frida -H <iPhone-IP>:${FRIDA_SERVER_PORT} -f com.xxx.xxx --no-pause"
        log_info "-------------------------------------------------"
    done

    modify_frida_tools
    log_success "frida-tools 修改完成"
    cd ..

}
initialize_config() {
    # 确保配置文件所在的目录存在
    mkdir -p "$(dirname "$CONFIG_FILE")"

    if [ ! -f "$CONFIG_FILE" ]; then
        echo "# Fridare Configuration File" > "$CONFIG_FILE"
        echo "FRIDA_SERVER_PORT=$DEF_FRIDA_SERVER_PORT" >> "$CONFIG_FILE"
        echo "CURL_PROXY=" >> "$CONFIG_FILE"
        echo "AUTO_CONFIRM=$DEF_AUTO_CONFIRM" >> "$CONFIG_FILE"
        echo "FRIDA_NAME=" >> "$CONFIG_FILE"
    fi
}
# 主函数
main() {
    initialize_config
    # 读取配置文件（如果存在）
    if [ -f "$CONFIG_FILE" ]; then
        source "$CONFIG_FILE"
    fi

    # 将配置文件中的值赋给脚本变量
    FRIDA_SERVER_PORT=${FRIDA_SERVER_PORT:-$DEF_FRIDA_SERVER_PORT}
    CURL_PROXY=${CURL_PROXY:-""}
    AUTO_CONFIRM=${AUTO_CONFIRM:-$DEF_AUTO_CONFIRM}
    FRIDA_NAME=${FRIDA_NAME:-""}

    # 解析参数
    parse_arguments "$@"
}
# 执行主函数
main "$@"
