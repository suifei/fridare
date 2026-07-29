package ui

import (
	"fmt"
	"fridare-gui/internal/assets"
	"fridare-gui/internal/config"
	"fridare-gui/internal/core"
	"fridare-gui/internal/utils"
	"log"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	WindowMinWidth  = 1200
	WindowMinHeight = 800
)

// LogEntry 自定义日志组件 - 模拟终端样式
type LogEntry struct {
	*widget.RichText
	logContent string
}

// NewLogEntry 创建新的日志组件
func NewLogEntry() *LogEntry {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapWord
	richText.Scroll = container.ScrollBoth

	log := &LogEntry{
		RichText:   richText,
		logContent: "",
	}

	// 设置初始样式和背景提示
	log.updateContent()

	return log
}

// updateContent 更新内容并设置样式
func (l *LogEntry) updateContent() {
	if l.logContent == "" {
		l.logContent = "📋 日志输出区域 (模拟终端样式)\n"
	}

	// 使用代码块样式来模拟终端外观
	l.RichText.ParseMarkdown("```\n" + l.logContent + "\n```")
}

// SetLogText 设置日志文本
func (l *LogEntry) SetLogText(text string) {
	l.logContent = text
	l.updateContent()
}

// AppendLogText 追加日志文本
func (l *LogEntry) AppendLogText(text string) {
	l.logContent += text
	l.updateContent()
}

// String 获取当前文本内容
func (l *LogEntry) String() string {
	return l.logContent
}

// MainWindow 主窗口结构
type MainWindow struct {
	app    fyne.App
	window fyne.Window
	config *config.Config

	// UI 组件
	content      *fyne.Container
	tabContainer *container.AppTabs
	statusBar    *widget.Label
	logText      *LogEntry // 改为自定义日志组件

	// 工具栏代理配置控件
	proxyEntry *FixedWidthEntry

	// 全局配置控件
	globalMagicNameEntry *FixedWidthEntry
	globalPortEntry      *FixedWidthEntry

	// 功能模块
	downloadTab *DownloadTab
	modifyTab   *ModifyTab
	packageTab  *PackageTab
	createTab   *CreateTab // 新增创建标签页
	toolsTab    *ToolsTab
	settingsTab *SettingsTab
	helpTab     *HelpTab     // 新增帮助标签页
	analysisTab *AnalysisTab // 新增分析标签页
}

// NewMainWindow 创建主窗口
func NewMainWindow(app fyne.App, cfg *config.Config) *MainWindow {
	// 创建窗口
	window := app.NewWindow("Fridare GUI - Frida 魔改工具")
	window.SetMaster()
	window.SetIcon(assets.AppIcon) // 设置窗口图标

	// 设置窗口大小
	window.Resize(fyne.NewSize(float32(cfg.WindowWidth), float32(cfg.WindowHeight)))
	window.SetFixedSize(false)

	// 设置最小尺寸
	window.SetContent(widget.NewLabel("Loading..."))

	// 设置窗口最小尺寸
	if cfg.WindowWidth < WindowMinWidth {
		cfg.WindowWidth = WindowMinWidth
	}
	if cfg.WindowHeight < WindowMinHeight {
		cfg.WindowHeight = WindowMinHeight
	}

	mw := &MainWindow{
		app:    app,
		window: window,
		config: cfg,
	}

	// 初始化UI
	mw.setupUI()

	// 应用主题
	mw.applyTheme()

	// 显示通知
	mw.showNoticeAsync()

	return mw
}

// setupUI 设置UI
func (mw *MainWindow) setupUI() {
	// 创建功能标签页
	mw.tabContainer = container.NewAppTabs()

	// 创建各个功能模块
	mw.downloadTab = NewDownloadTab(mw.app, mw.config, mw.updateStatus)
	mw.modifyTab = NewModifyTab(mw.app, mw.config, mw.updateStatus, mw.addLog)
	mw.packageTab = NewPackageTab(mw.app, mw.config, mw.updateStatus, mw.addLog)
	mw.createTab = NewCreateTab(mw.app, mw.config, mw.updateStatus, mw.addLog) // 新增创建标签页
	mw.toolsTab = NewToolsTab(mw.config, mw.updateStatus)
	mw.toolsTab.SetLogFunction(mw.addLog) // 设置日志函数
	mw.settingsTab = NewSettingsTab(mw.config, mw.updateStatus, mw.applyTheme, mw.window)
	mw.helpTab = NewHelpTab()                                                      // 新增帮助标签页
	mw.analysisTab = NewAnalysisTab(mw.app, mw.config, mw.updateStatus, mw.addLog) // 新增分析标签页

	// 添加标签页（与原型保持一致），为每个tab添加滚动支持
	mw.tabContainer.Append(container.NewTabItem("📥 下载",
		container.NewScroll(mw.downloadTab.Content())))
	mw.tabContainer.Append(container.NewTabItem("🔧 frida 魔改",
		container.NewScroll(mw.modifyTab.Content())))
	mw.tabContainer.Append(container.NewTabItem("📦 iOS DEB 魔改",
		container.NewScroll(mw.packageTab.Content())))
	mw.tabContainer.Append(container.NewTabItem("🆕 iOS DEB 打包",
		container.NewScroll(mw.createTab.Content()))) // 新增创建标签页
	mw.tabContainer.Append(container.NewTabItem("🛠️ frida-tools 魔改",
		container.NewScroll(mw.toolsTab.Content())))
	mw.tabContainer.Append(container.NewTabItem("🔬 文件分析",
		container.NewScroll(mw.analysisTab.Content()))) // 新增分析标签页
	mw.tabContainer.Append(container.NewTabItem("⚙️ 设置",
		container.NewScroll(mw.settingsTab.Content()))) // 设置标签页
	mw.tabContainer.Append(container.NewTabItem("❓ 帮助",
		mw.helpTab.Content())) // 帮助标签页 - 不需要滚动包装因为内部已处理
	mw.tabContainer.OnSelected = func(tab *container.TabItem) {
		mw.updateStatus("切换到标签: " + tab.Text)
		// 如果当前标签是设置标签页，则刷新配置显示
		if tab.Text == "⚙️ 设置" {
			mw.settingsTab.RefreshConfigDisplay()
		}
	}

	// 创建底部状态区域（包含日志和按钮）
	bottomArea := mw.createBottomArea()

	// 创建顶部工具栏
	toolbar := mw.createToolbar()

	// 创建主布局 - 简化为垂直布局，移除左侧边栏
	mw.content = container.NewBorder(
		toolbar,         // top
		bottomArea,      // bottom
		nil,             // left
		nil,             // right
		mw.tabContainer, // center
	)

	// 设置窗口内容
	mw.window.SetContent(mw.content)
}

// createToolbar 创建工具栏
func (mw *MainWindow) createToolbar() *fyne.Container {
	// Logo图标 - 使用canvas.Image并设置固定大小
	logoImage := canvas.NewImageFromResource(assets.AppIcon)
	logoImage.FillMode = canvas.ImageFillOriginal
	logoImage.Resize(fyne.NewSize(64, 64))
	logoImage.SetMinSize(fyne.NewSize(64, 64))

	// 应用标题
	titleLabel := widget.NewLabel("Fridare GUI - Frida 魔改工具")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// 代理配置 - 使用固定宽度的Entry，参考download_tab的实现
	mw.proxyEntry = NewFixedWidthEntry(300)
	mw.proxyEntry.SetPlaceHolder("http://proxy:port (可选)")
	if mw.config.Proxy != "" {
		mw.proxyEntry.SetText(mw.config.Proxy)
	}

	// 代理测试按钮
	proxyTestBtn := widget.NewButtonWithIcon("", theme.SearchIcon(), func() {
		mw.testProxy()
	})
	proxyTestBtn.SetText("测试")

	// 保存代理配置按钮
	proxySaveBtn := widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), func() {
		mw.saveProxyConfig()
	})
	proxySaveBtn.SetText("保存")

	// 全局魔改配置区域
	mw.globalMagicNameEntry = NewFixedWidthEntry(80)
	mw.globalMagicNameEntry.SetPlaceHolder("5字符")
	if mw.config.MagicName != "" {
		mw.globalMagicNameEntry.SetText(mw.config.MagicName)
	} else {
		mw.globalMagicNameEntry.SetText("frida")
	}

	mw.globalPortEntry = NewFixedWidthEntry(60)
	mw.globalPortEntry.SetPlaceHolder("端口")
	if mw.config.DefaultPort > 0 {
		mw.globalPortEntry.SetText(fmt.Sprintf("%d", mw.config.DefaultPort))
	} else {
		mw.globalPortEntry.SetText("27042")
	}

	// 全局配置验证和保存
	mw.globalMagicNameEntry.OnChanged = func(text string) {
		if len(text) == 5 && isValidMagicName(text) {
			mw.updateGlobalMagicName(text)
		}
	}

	mw.globalPortEntry.OnChanged = func(text string) {
		if port, err := strconv.Atoi(text); err == nil && port > 0 && port <= 65535 {
			mw.updateGlobalPort(port)
		}
	}

	// 随机魔改名称按钮
	randomMagicBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		randomName := utils.GenerateRandomName()
		mw.globalMagicNameEntry.SetText(randomName)
		mw.updateGlobalMagicName(randomName)
	})
	randomMagicBtn.SetText("随机")

	// 代理配置区域 - 参考download_tab的布局方式
	proxyArea := container.NewHBox(
		widget.NewLabel("代理:"),
		mw.proxyEntry,
		proxyTestBtn,
		proxySaveBtn,
		widget.NewLabel("全局魔改:"),
		mw.globalMagicNameEntry,
		randomMagicBtn,
		widget.NewLabel("端口:"),
		mw.globalPortEntry,
	)

	// 帮助按钮
	helpBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		mw.showAbout()
	})
	logoTitle := container.NewHBox(
		logoImage,
		titleLabel) // 左侧: Logo + Title
	// 工具栏布局 - 分两行显示
	topRow := container.NewBorder(
		nil, nil,
		logoTitle, // 左侧: Logo + Title
		helpBtn,   // 右侧: 帮助按钮
		proxyArea, // 中间: 代理配置
	)

	toolbar := container.NewVBox(topRow)

	return toolbar
}

// updateStatus 更新状态栏
func (mw *MainWindow) updateStatus(message string) {
	if mw.statusBar != nil {
		fyne.Do(func() {
			mw.statusBar.SetText(message)
		})
	}
	// 记录日志但不立即更新UI
	log.Println("STATUS:", message)
}

// saveProxyConfig 保存代理配置
func (mw *MainWindow) saveProxyConfig() {
	// 更新配置
	mw.config.Proxy = mw.proxyEntry.Text

	// 保存配置
	if err := mw.config.Save(); err != nil {
		mw.updateStatus("保存代理配置失败: " + err.Error())
		mw.addLog("ERROR: 保存代理配置失败: " + err.Error())
	} else {
		mw.updateStatus("代理配置已保存")
		mw.addLog("INFO: 代理配置已保存")
	}
}

// saveConfig 保存配置
func (mw *MainWindow) saveConfig() {
	if err := mw.config.Save(); err != nil {
		mw.updateStatus("保存配置失败: " + err.Error())
		log.Printf("保存配置失败: %v", err)
	} else {
		mw.updateStatus("配置已保存")
	}
}

// applyTheme 应用主题
func (mw *MainWindow) applyTheme() {
	switch mw.config.Theme {
	case "dark":
		mw.app.Settings().SetTheme(theme.DarkTheme())
	case "light":
		mw.app.Settings().SetTheme(theme.LightTheme())
	default:
		// auto - 使用系统默认
		mw.app.Settings().SetTheme(theme.DefaultTheme())
	}
}

// showAbout 显示关于对话框
func (mw *MainWindow) showAbout() {
	// 创建简单的对话框内容
	content := widget.NewLabel(`Fridare GUI v4.0.2

Frida 重打包和修补工具的图形界面版本

特性: 下载发行版, 二进制修补, DEB包生成, 工具集成

作者: suifei@gmail.com
项目: https://github.com/suifei/fridare`)

	content.Alignment = fyne.TextAlignCenter
	content.Wrapping = fyne.TextWrapWord

	// 创建对话框
	dialog := dialog.NewCustom("关于 Fridare GUI", "确定", content, mw.window)
	dialog.Resize(fyne.NewSize(400, 250))
	dialog.Show()
}

// showNotice 显示通知对话框
func (mw *MainWindow) showNotice() {
	// 创建简单的对话框内容, 支持多行文本和markdown
	// 通知内容从 https://raw.githubusercontent.com/suifei/fridare/main/NOTICE.md 获取
	// 网络请求失败则不显示(自动挂接代理)

	// 从配置获取代理，如果配置没有，则尝试获取系统代理  ，否则为“”
	// 系统代理获取：
	// HTTPProxy:  getEnvAny("HTTP_PROXY", "http_proxy"),
	// HTTPSProxy: getEnvAny("HTTPS_PROXY", "https_proxy"),
	// NoProxy:    getEnvAny("NO_PROXY", "no_proxy"),
	// CGI:        os.Getenv("REQUEST_METHOD") != "",
	noticeURL := "https://raw.githubusercontent.com/suifei/fridare/main/NOTICE.md"
	noticeContent, err := utils.FetchRemoteText(
		noticeURL,
		mw.config.Proxy)
	if err != nil || strings.TrimSpace(noticeContent) == "" {
		// 获取失败或内容为空则不显示通知
		return
	} else {
		mw.addLog("INFO: 成功获取通知内容: " + noticeURL)
		if mw.config.NoShowNotice {
			// 将通知显示到log中不弹窗，markdown 文本用于日志显示
			mw.addLog("NOTICE: " + strings.ReplaceAll(noticeContent, "\n\n", "\n"))
			mw.addLog("INFO: 配置设置为不显示通知，跳过显示")
			return
		}
	}

	// 创建对话框
	contentViewer := widget.NewRichText()
	contentViewer.ParseMarkdown(noticeContent)
	contentViewer.Wrapping = fyne.TextWrapWord
	// 支持对话框勾选不再显示并记录到配置文件
	checkbox := widget.NewCheck("不再显示此通知", func(checked bool) {
		mw.config.NoShowNotice = checked
		// 保存配置
		if err := mw.config.Save(); err != nil {
			mw.updateStatus("保存配置失败: " + err.Error())
			mw.addLog("ERROR: 保存配置失败: " + err.Error())
		} else {
			mw.updateStatus("配置已保存")
			mw.addLog("INFO: 配置已保存")
		}
	})
	checkbox.SetChecked(mw.config.NoShowNotice)

	content := container.NewBorder(nil, checkbox, nil, nil, container.NewVScroll(contentViewer))

	dialog := dialog.NewCustom("Fridare GUI - 通知", "确定", content, mw.window)
	dialog.Resize(fyne.NewSize(400, 400))
	dialog.Show()
}

// ShowAndRun 显示窗口并运行应用
func (mw *MainWindow) ShowAndRun() {
	// 设置关闭回调
	mw.window.SetCloseIntercept(func() {
		mw.saveConfig()
		mw.app.Quit()
	})

	// 显示窗口
	mw.window.Show()

	// 运行应用
	mw.app.Run()
}

// StatusUpdater 状态更新接口
type StatusUpdater func(message string)

// createBottomArea 创建底部区域
func (mw *MainWindow) createBottomArea() *fyne.Container {
	// 创建状态栏
	mw.statusBar = widget.NewLabel("等待操作...")
	mw.statusBar.TextStyle = fyne.TextStyle{Italic: true}

	// 创建日志区域 - 使用自定义日志组件，黑色背景绿色文字
	mw.logText = NewLogEntry()
	mw.logText.Resize(fyne.NewSize(0, 60)) // 设置高度

	// 创建日志控制按钮
	clearBtn := widget.NewButton("清空", func() {
		mw.logText.SetLogText("")
		mw.updateStatus("日志已清空")
	})

	historyBtn := widget.NewButton("历史", func() {
		mw.updateStatus("历史功能待实现")
	})

	logControls := container.NewHBox(
		mw.statusBar,
		widget.NewSeparator(),
		clearBtn,
		historyBtn,
	)

	// 创建带滚动的日志区域
	logScroll := container.NewScroll(mw.logText)
	logScroll.SetMinSize(fyne.NewSize(0, 60))

	// 组装底部区域
	bottomArea := container.NewBorder(
		logControls, // top
		nil,         // bottom
		nil,         // left
		nil,         // right
		logScroll,   // center
	)

	return bottomArea
}

// addLog 添加日志
func (mw *MainWindow) addLog(message string) {
	if mw.logText != nil {
		timestamp := time.Now().Format("15:04:05")
		logEntry := fmt.Sprintf("[%s] %s", timestamp, message)
		fyne.Do(func() {
			mw.logText.AppendLogText(logEntry + "\n")
		})
	}
}

// testProxy 测试代理连接
func (mw *MainWindow) testProxy() {
	proxyURL := strings.TrimSpace(mw.proxyEntry.Text)

	// 如果代理为空，测试直连
	if proxyURL == "" {
		mw.updateStatus("正在测试直连...")
		mw.addLog("INFO: 开始测试直连")
	} else {
		mw.updateStatus("正在测试代理连接...")
		mw.addLog("INFO: 开始测试代理连接: " + proxyURL)
	}

	// 异步执行测试
	go func() {
		// 测试多个URL
		testURLs := []struct {
			name string
			url  string
		}{
			{"GitHub Frida API", "https://api.github.com/repos/frida/frida/releases/latest"},
			{"Google", "https://www.google.com"},
		}

		var results []string
		var successCount int

		for _, test := range testURLs {
			success, message, err := utils.TestProxy(proxyURL, test.url, mw.config.Timeout)

			if success {
				results = append(results, fmt.Sprintf("✓ %s: %s", test.name, message))
				successCount++
				mw.addLog(fmt.Sprintf("SUCCESS: %s - %s", test.name, message))
			} else {
				results = append(results, fmt.Sprintf("✗ %s: %s", test.name, message))
				mw.addLog(fmt.Sprintf("ERROR: %s - %s", test.name, message))
				if err != nil {
					mw.addLog("ERROR: " + err.Error())
				}
			}
		}
		ProxyInfo := "Proxy: Direct"
		if proxyURL != "" {
			ProxyInfo = "Proxy: " + proxyURL
		}
		// 更新UI
		if successCount > 0 {
			if successCount == len(testURLs) {
				mw.updateStatus("代理测试完全成功")
				dialog.ShowInformation("代理测试结果",
					fmt.Sprintf("%s, 测试成功！(%d/%d)\n\n%s",
						ProxyInfo,
						successCount, len(testURLs), strings.Join(results, "\n")),
					mw.window)
			} else {
				mw.updateStatus(fmt.Sprintf("代理测试部分成功 (%d/%d)", successCount, len(testURLs)))
				dialog.ShowInformation("代理测试结果",
					fmt.Sprintf("%s, 部分成功 (%d/%d)\n\n%s",
						ProxyInfo,
						successCount, len(testURLs), strings.Join(results, "\n")),
					mw.window)
			}
		} else {
			mw.updateStatus("代理测试失败")
			dialog.ShowError(
				fmt.Errorf("%s, 代理测试失败\n\n%s", ProxyInfo, strings.Join(results, "\n")),
				mw.window)
		}
	}()
}

// isValidMagicName 验证魔改名称（与 core.ValidateMagicName / hexreplace 一致：5×a-z）
func isValidMagicName(name string) bool {
	return core.ValidateMagicName(name) == nil
}

// updateGlobalMagicName 更新全局魔改名称
func (mw *MainWindow) updateGlobalMagicName(magicName string) {
	mw.config.MagicName = magicName
	mw.saveProxyConfig() // 重用现有的保存方法

	// 通知所有标签页更新
	mw.updateTabsGlobalConfig()
	mw.updateStatus("全局魔改名称已更新: " + magicName)
}

// updateGlobalPort 更新全局端口
func (mw *MainWindow) updateGlobalPort(port int) {
	mw.config.DefaultPort = port
	mw.saveProxyConfig() // 重用现有的保存方法

	// 通知所有标签页更新
	mw.updateTabsGlobalConfig()
	mw.updateStatus(fmt.Sprintf("全局端口已更新: %d", port))
}

// updateTabsGlobalConfig 更新所有标签页的全局配置
func (mw *MainWindow) updateTabsGlobalConfig() {
	// 更新ModifyTab
	if mw.modifyTab != nil {
		mw.modifyTab.UpdateGlobalConfig(mw.config.MagicName, mw.config.DefaultPort)
	}

	// 更新PackageTab
	if mw.packageTab != nil {
		mw.packageTab.UpdateGlobalConfig(mw.config.MagicName, mw.config.DefaultPort)
	}

	// 更新CreateTab
	if mw.createTab != nil {
		mw.createTab.UpdateGlobalConfig(mw.config.MagicName, mw.config.DefaultPort)
	}

	// 更新ToolsTab
	if mw.toolsTab != nil {
		mw.toolsTab.UpdateGlobalConfig(mw.config.MagicName, mw.config.DefaultPort)
	}
}
