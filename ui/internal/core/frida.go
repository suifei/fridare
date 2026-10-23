package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fridare-gui/internal/i18n"

	"github.com/go-resty/resty/v2"
)

// FridaVersion Frida版本信息
type FridaVersion struct {
	Version    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	Published  time.Time `json:"published_at"`
	PreRelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []Asset   `json:"assets"`
}

// Asset 发布资源
type Asset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
	ContentType string `json:"content_type"`
}

// Platform 支持的平台
type Platform struct {
	OS   string
	Arch string
	Name string
}

// SupportedPlatforms 支持的平台列表
var SupportedPlatforms = []Platform{
	{"android", "arm64", "Android ARM64"},
	{"android", "arm", "Android ARM"},
	{"android", "x86_64", "Android x86_64"},
	{"android", "x86", "Android x86"},
	{"ios", "arm64", "iOS ARM64"},
	{"windows", "x86_64", "Windows x64"},
	{"windows", "x86", "Windows x86"},
	{"linux", "x86_64", "Linux x64"},
	{"linux", "arm64", "Linux ARM64"},
	{"macos", "arm64", "macOS ARM64"},
	{"macos", "x86_64", "macOS x64"},
}

// FileType 文件类型
type FileType string

const (
	FileTypeServer FileType = "server"
	FileTypeGadget FileType = "gadget"
	FileTypeTools  FileType = "tools"
)

// FridaClient Frida客户端
type FridaClient struct {
	client *resty.Client
	proxy  string
}

// NewFridaClient 创建新的Frida客户端
func NewFridaClient(proxy string, timeout time.Duration) *FridaClient {
	client := resty.New().
		SetTimeout(timeout).
		SetRetryCount(3).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(30 * time.Second)

	if proxy != "" {
		client.SetProxy(proxy)
	}

	// 设置用户代理
	client.SetHeader("User-Agent", "Fridare-GUI/1.0.0")

	return &FridaClient{
		client: client,
		proxy:  proxy,
	}
}

// GetVersions 获取Frida版本列表
func (fc *FridaClient) GetVersions() ([]FridaVersion, error) {
	var releases []FridaVersion

	// GitHub API获取releases
	resp, err := fc.client.R().
		SetResult(&releases).
		Get("https://api.github.com/repos/frida/frida/releases")

	if err != nil {
		return nil, fmt.Errorf(i18n.T("获取版本列表失败: %w"), err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf(i18n.T("GitHub API返回错误状态码: %d"), resp.StatusCode())
	}

	// 过滤并排序
	var validReleases []FridaVersion
	for _, release := range releases {
		if !release.Draft && len(release.Assets) > 0 {
			validReleases = append(validReleases, release)
		}
	}

	// 按版本号排序（最新的在前面）
	sort.Slice(validReleases, func(i, j int) bool {
		return compareVersions(validReleases[i].Version, validReleases[j].Version) > 0
	})

	return validReleases, nil
}

// GetLatestVersion 获取最新版本
func (fc *FridaClient) GetLatestVersion() (*FridaVersion, error) {
	versions, err := fc.GetVersions()
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf(i18n.T("没有找到可用版本"))
	}

	// 返回第一个非预发布版本
	for _, version := range versions {
		if !version.PreRelease {
			return &version, nil
		}
	}

	// 如果没有稳定版本，返回最新的预发布版本
	return &versions[0], nil
}

// FindAsset 查找匹配的资源文件
func (fc *FridaClient) FindAsset(version *FridaVersion, platform Platform, fileType FileType) (*Asset, error) {
	var pattern string

	switch fileType {
	case FileTypeServer:
		pattern = fmt.Sprintf(`frida-server-.*-%s-%s.*`, version.Version, platform.OS)
		if platform.Arch != "" {
			pattern = fmt.Sprintf(`frida-server-.*-%s-%s-%s.*`, version.Version, platform.OS, platform.Arch)
		}
	case FileTypeGadget:
		pattern = fmt.Sprintf(`frida-gadget-.*-%s-%s.*`, version.Version, platform.OS)
		if platform.Arch != "" {
			pattern = fmt.Sprintf(`frida-gadget-.*-%s-%s-%s.*`, version.Version, platform.OS, platform.Arch)
		}
	case FileTypeTools:
		// PyPI project is frida-tools; wheel/sdist basenames are frida_tools-* or frida-tools-*.
		// The tools line has its own version, so do not require the frida server version in the name.
		pattern = `(?i)(^|/)frida[-_]tools-`
	default:
		return nil, fmt.Errorf(i18n.T("不支持的文件类型: %s"), fileType)
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("编译正则表达式失败: %w"), err)
	}

	for _, asset := range version.Assets {
		if regex.MatchString(asset.Name) {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf(i18n.T("未找到匹配的文件: %s"), pattern)
}

// DownloadProgress 下载进度回调
type DownloadProgress func(downloaded, total int64, speed float64)

// DownloadFile 下载文件
func (fc *FridaClient) DownloadFile(url, filename string, progress DownloadProgress) error {
	resp, err := fc.client.R().
		SetDoNotParseResponse(true).
		Get(url)

	if err != nil {
		return fmt.Errorf(i18n.T("下载失败: %w"), err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() != 200 {
		return fmt.Errorf(i18n.T("下载失败，状态码: %d"), resp.StatusCode())
	}

	// 获取文件大小
	contentLength := resp.Header().Get("Content-Length")
	var totalSize int64
	if contentLength != "" {
		totalSize, _ = strconv.ParseInt(contentLength, 10, 64)
	}

	// 创建文件
	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf(i18n.T("创建文件失败: %w"), err)
	}
	defer out.Close()

	// 下载并监控进度
	var downloaded int64
	startTime := time.Now()
	buffer := make([]byte, 32*1024) // 32KB缓冲区

	for {
		n, err := resp.RawBody().Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf(i18n.T("写入文件失败: %w"), writeErr)
			}

			downloaded += int64(n)

			// 计算速度
			elapsed := time.Since(startTime).Seconds()
			speed := float64(downloaded) / elapsed

			// 调用进度回调
			if progress != nil {
				progress(downloaded, totalSize, speed)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf(i18n.T("读取响应失败: %w"), err)
		}
	}

	return nil
}

// DownloadFileWithContext 下载文件（支持上下文取消）
func (fc *FridaClient) DownloadFileWithContext(ctx context.Context, url, filename string, progress DownloadProgress) error {
	resp, err := fc.client.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(url)

	if err != nil {
		return fmt.Errorf(i18n.T("下载失败: %w"), err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() != 200 {
		return fmt.Errorf(i18n.T("下载失败，状态码: %d"), resp.StatusCode())
	}

	// 获取文件大小
	contentLength := resp.Header().Get("Content-Length")
	var totalSize int64
	if contentLength != "" {
		totalSize, _ = strconv.ParseInt(contentLength, 10, 64)
	}

	// 创建文件
	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf(i18n.T("创建文件失败: %w"), err)
	}
	defer out.Close()

	// 下载并监控进度
	var downloaded int64
	startTime := time.Now()
	buffer := make([]byte, 32*1024) // 32KB缓冲区

	for {
		// 更频繁地检查上下文是否被取消
		select {
		case <-ctx.Done():
			// 立即关闭文件并删除部分下载的文件
			out.Close()
			os.Remove(filename)
			return ctx.Err()
		default:
		}

		// 设置读取超时，使得取消检查更及时
		n, err := resp.RawBody().Read(buffer)
		if n > 0 {
			// 在写入前再次检查取消
			select {
			case <-ctx.Done():
				out.Close()
				os.Remove(filename)
				return ctx.Err()
			default:
			}

			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf(i18n.T("写入文件失败: %w"), writeErr)
			}

			downloaded += int64(n)

			// 计算速度
			elapsed := time.Since(startTime).Seconds()
			speed := float64(downloaded) / elapsed

			// 调用进度回调（在回调中也检查取消）
			if progress != nil {
				select {
				case <-ctx.Done():
					out.Close()
					os.Remove(filename)
					return ctx.Err()
				default:
					progress(downloaded, totalSize, speed)
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf(i18n.T("读取响应失败: %w"), err)
		}
	}

	return nil
}

// compareVersions 比较版本号
func compareVersions(v1, v2 string) int {
	// 移除 'v' 前缀
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			p2, _ = strconv.Atoi(parts2[i])
		}

		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}

	return 0
}

// GetPlatformByName 根据名称获取平台
func GetPlatformByName(name string) *Platform {
	for _, platform := range SupportedPlatforms {
		if platform.Name == name {
			return &platform
		}
	}
	return nil
}

// GetFileTypeByName 根据名称获取文件类型
func GetFileTypeByName(name string) FileType {
	switch name {
	case "frida-server (可执行文件)":
		return FileTypeServer
	case "frida-gadget (动态库)":
		return FileTypeGadget
	case "frida-tools (Python包)":
		return FileTypeTools
	default:
		return FileTypeServer
	}
}

// FormatSize 格式化文件大小
func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed 格式化下载速度
func FormatSpeed(bytesPerSecond float64) string {
	return FormatSize(int64(bytesPerSecond)) + "/s"
}
