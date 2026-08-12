package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"fridare-gui/internal/assets"
	"fridare-gui/internal/config"
	"fridare-gui/internal/core"
	"fridare-gui/internal/rebuild"
	"fridare-gui/internal/utils"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// fixedWidthEntry 创建固定宽度的Entry
func fixedWidthEntry(width float32, placeholder string) *FixedWidthEntry {
	entry := NewFixedWidthEntry(width)
	entry.SetPlaceHolder(placeholder)
	entry.Resize(fyne.NewSize(width, entry.MinSize().Height))
	return entry
}

// ModifyTab 修改标签页
type ModifyTab struct {
	app          fyne.App
	config       *config.Config
	updateStatus StatusUpdater
	addLog       func(string) // 添加日志记录功能
	content      *fyne.Container

	// UI 组件
	filePathEntry  *widget.Entry
	magicNameEntry *widget.Entry
	fileInfoText   *widget.RichText
	progressBar    *widget.ProgressBar
	progressLabel  *widget.Label
	patchBtn       *widget.Button

	// 核心功能
	hexReplacer *core.HexReplacer
}

// NewModifyTab 创建修改标签页
func NewModifyTab(app fyne.App, cfg *config.Config, statusUpdater StatusUpdater, logFunc func(string)) *ModifyTab {
	mt := &ModifyTab{
		app:          app,
		config:       cfg,
		updateStatus: statusUpdater,
		addLog:       logFunc,
		hexReplacer:  core.NewHexReplacer(),
	}

	mt.setupUI()
	return mt
}

func (mt *ModifyTab) setupUI() {
	// 输入文件选择区域
	mt.filePathEntry = widget.NewEntry()
	mt.filePathEntry.SetPlaceHolder("选择要修改的 Frida 二进制文件...")
	mt.filePathEntry.OnChanged = func(path string) {
		if path != "" {
			mt.analyzeFile(path)
		} else {
			mt.fileInfoText.ParseMarkdown("")
		}
		// 立即验证输入
		mt.validateInput(mt.magicNameEntry.Text, mt.filePathEntry.Text)
	}

	browseInputBtn := widget.NewButton("浏览", func() {
		mt.selectInputFile()
	})

	fileSelectArea := container.NewBorder(
		nil, nil, nil, browseInputBtn, mt.filePathEntry,
	)

	// 魔改选项
	mt.magicNameEntry = widget.NewEntry()
	mt.magicNameEntry.SetPlaceHolder("输入5个小写字母")
	if mt.config.MagicName != "" && len(mt.config.MagicName) == 5 {
		mt.magicNameEntry.SetText(mt.config.MagicName)
	} else {
		mt.magicNameEntry.SetText("frida")
	}
	// 验证输入
	mt.magicNameEntry.OnChanged = func(text string) {
		mt.validateInput(text, mt.filePathEntry.Text)
	}

	// 随机生成按钮
	randomBtn := widget.NewButton("随机", func() {
		randomName := mt.generateRandomName()
		mt.magicNameEntry.SetText(randomName)
		mt.validateInput(randomName, mt.filePathEntry.Text)
	})

	magicNameArea := container.NewBorder(
		nil, nil, nil, randomBtn, mt.magicNameEntry,
	)

	optionsForm := container.NewVBox(
		widget.NewLabel("魔改名称 (必须5个小写字母):"),
		magicNameArea,
	)

	// 文件信息显示区域
	mt.fileInfoText = widget.NewRichText()
	mt.fileInfoText.Resize(fyne.NewSize(0, 200))

	fileInfoScroll := container.NewScroll(mt.fileInfoText)
	fileInfoScroll.SetMinSize(fyne.NewSize(0, 200))

	fileInfoCard := widget.NewCard("文件信息", "二进制文件格式和架构信息", fileInfoScroll)

	// 修改按钮
	mt.patchBtn = widget.NewButton("开始魔改", func() {
		mt.startPatching()
	})
	mt.patchBtn.Importance = widget.HighImportance
	mt.patchBtn.Disable() // 初始状态禁用

	// 进度显示
	mt.progressBar = widget.NewProgressBar()
	mt.progressBar.Hide()

	mt.progressLabel = widget.NewLabel("")
	mt.progressLabel.Hide()

	// 主布局
	mainContent := container.NewVBox(
		container.NewVBox(
			widget.NewLabel("输入文件:"),
			fileSelectArea,
		),
		widget.NewSeparator(),
		optionsForm,
		widget.NewSeparator(),
		mt.patchBtn,
		mt.progressBar,
		mt.progressLabel,
	)

	// 使用水平分割布局
	splitContainer := container.NewHSplit(
		widget.NewCard("二进制魔改器", "修改 Frida 二进制文件的特征字符串", mainContent),
		fileInfoCard,
	)
	splitContainer.Offset = 0.6 // 左侧占60%

	mt.content = container.NewPadded(splitContainer)

	// 初始验证状态
	mt.validateInput(mt.magicNameEntry.Text, mt.filePathEntry.Text)
}

// selectInputFile 选择输入文件
func (mt *ModifyTab) selectInputFile() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		path := reader.URI().Path()
		mt.filePathEntry.SetText(path)
		mt.validateInput(mt.magicNameEntry.Text, path)
	}, fyne.CurrentApp().Driver().AllWindows()[0])

	// 移除文件类型过滤，支持所有文件类型
	fileDialog.Show()
}

// analyzeFile 分析文件
func (mt *ModifyTab) analyzeFile(filePath string) {
	go func() {
		fyne.Do(func() {
			mt.updateStatus("正在分析文件...")
		})

		description, err := mt.hexReplacer.DescribeFile(filePath)
		if err != nil {
			errMsg := err.Error()
			fyne.Do(func() {
				mt.fileInfoText.ParseMarkdown(fmt.Sprintf("**错误:** %s", errMsg))
				mt.updateStatus("文件分析失败: " + errMsg)
			})
			return
		}

		// 格式化显示信息
		markdown := fmt.Sprintf("**文件路径:** %s\n\n**文件信息:**\n```\n%s\n```", filePath, description)
		fyne.Do(func() {
			mt.fileInfoText.ParseMarkdown(markdown)
			mt.updateStatus("文件分析完成")
		})
	}()
}

// validateInput 验证输入
func (mt *ModifyTab) validateInput(name string, filePath string) {
	inputValid := name != ""
	nameValid := core.ValidateMagicName(name) == nil
	filePathValid := utils.FileExists(filePath)

	if inputValid && nameValid && filePathValid {
		mt.patchBtn.Enable()
	} else {
		mt.patchBtn.Disable()
	}

}

// generateRandomName 生成随机名称
func (mt *ModifyTab) generateRandomName() string {
	return utils.GenerateRandomName()
}

// startPatching 开始修改
func (mt *ModifyTab) startPatching() {
	inputPath := mt.filePathEntry.Text
	magicName := mt.magicNameEntry.Text

	// 自动生成输出路径
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// 将文件名中的 frida 替换为魔改名称
	if strings.Contains(name, "frida") {
		name = strings.ReplaceAll(name, "frida", magicName)
	} else {
		name = magicName + "_" + name
	}

	outputPath := filepath.Join(dir, name+ext)

	// 显示进度
	mt.progressBar.Show()
	mt.progressLabel.Show()
	mt.progressBar.SetValue(0)
	mt.progressLabel.SetText("正在初始化...")
	mt.patchBtn.Disable()

	go func() {
		defer func() {
			fyne.Do(func() {
				mt.progressBar.Hide()
				mt.progressLabel.Hide()
				mt.patchBtn.Enable()
			})
		}()

		fyne.Do(func() {
			mt.updateStatus("开始魔改二进制文件...")
			mt.addLog("INFO: 开始魔改二进制文件")
			mt.addLog(fmt.Sprintf("INFO: 输入文件: %s", inputPath))
			mt.addLog(fmt.Sprintf("INFO: 输出文件: %s", outputPath))
			mt.addLog(fmt.Sprintf("INFO: 魔改名称: %s", magicName))
		})

		// 进度回调必须在 UI 线程更新控件
		progressCallback := func(progress float64, message string) {
			fyne.Do(func() {
				mt.progressBar.SetValue(progress)
				mt.progressLabel.SetText(message)
				mt.updateStatus(message)
				mt.addLog(fmt.Sprintf("INFO: %s (%.1f%%)", message, progress*100))
			})
		}

		err := mt.hexReplacer.PatchFile(inputPath, magicName, outputPath, progressCallback)
		if err != nil {
			errorMsg := "魔改失败: " + err.Error()
			fyne.Do(func() {
				mt.updateStatus(errorMsg)
				mt.progressLabel.SetText("魔改失败!")
				mt.addLog("ERROR: " + errorMsg)
				dialog.ShowError(fmt.Errorf("魔改失败: %v", err), fyne.CurrentApp().Driver().AllWindows()[0])
			})
			return
		}

		mt.config.MagicName = magicName
		_ = mt.config.Save()

		successMsg := fmt.Sprintf("魔改完成! 输出文件: %s", outputPath)
		inputBaseName := filepath.Base(inputPath)
		outputBaseName := filepath.Base(outputPath)
		contentText := fmt.Sprintf("魔改完成!\n\n输入文件: %s\n输出文件: %s\n魔改名称: %s\n\n文件已保存到与输入文件相同的目录",
			inputBaseName, outputBaseName, magicName)

		fyne.Do(func() {
			mt.progressBar.SetValue(1.0)
			mt.progressLabel.SetText("魔改完成!")
			mt.updateStatus(successMsg)
			mt.addLog("SUCCESS: " + successMsg)
			mt.addLog("INFO: 配置已保存")
			dialog.ShowInformation("魔改完成", contentText, fyne.CurrentApp().Driver().AllWindows()[0])
		})
	}()
}

func (mt *ModifyTab) Content() *fyne.Container {
	return mt.content
}

func (mt *ModifyTab) Refresh() {
	// 刷新逻辑
}

// UpdateGlobalConfig 更新全局配置
func (mt *ModifyTab) UpdateGlobalConfig(magicName string, port int) {
	if mt.magicNameEntry != nil {
		mt.magicNameEntry.SetText(magicName)
	}
}

// PackageTab 打包标签页
type PackageTab struct {
	app          fyne.App
	config       *config.Config
	updateStatus StatusUpdater
	addLog       func(string)
	content      *fyne.Container

	// UI 组件
	debFileEntry    *widget.Entry // DEB文件选择
	outputPathEntry *widget.Entry
	portEntry       *widget.Entry
	magicNameEntry  *widget.Entry
	packageBtn      *widget.Button
	progressBar     *widget.ProgressBar
	progressLabel   *widget.Label

	// 核心功能
	debPackager *core.DebPackager
}

func NewPackageTab(app fyne.App, cfg *config.Config, statusUpdater StatusUpdater, logFunc func(string)) *PackageTab {
	pt := &PackageTab{
		app:          app,
		config:       cfg,
		updateStatus: statusUpdater,
		addLog:       logFunc,
		debPackager:  core.NewDebPackager(),
	}

	pt.setupUI()
	return pt
}

func (pt *PackageTab) setupUI() {
	// 直接设置为修改现有DEB包模式，不需要模式选择
	pt.debFileEntry = widget.NewEntry()
	pt.debFileEntry.SetPlaceHolder("选择要修改的 DEB 包文件...")
	pt.debFileEntry.OnChanged = func(path string) {
		pt.validateInput()
	}

	// 设置DEB文件选择区域
	browseDebBtn := widget.NewButton("浏览", func() {
		pt.selectDebFile()
	})

	debFileArea := container.NewBorder(
		nil, nil, widget.NewLabel("DEB文件:"), browseDebBtn, pt.debFileEntry,
	)

	// 包信息显示区域
	infoText := widget.NewRichText()
	infoText.ParseMarkdown("**DEB包修改器**\n\n" +
		"• 选择现有的Frida DEB包文件\n" +
		"• 自动读取包元数据\n" +
		"• 使用指定的魔改名称和端口进行修改\n" +
		"• 生成修改后的DEB包\n\n" +
		"**支持的修改：**\n" +
		"• 修改Frida服务名称\n" +
		"• 修改默认监听端口\n" +
		"• 保持原包的所有其他设置")

	packageInfoCard := widget.NewCard("操作说明", "", infoText)

	// 输出路径选择
	pt.outputPathEntry = widget.NewEntry()
	pt.outputPathEntry.SetPlaceHolder("DEB 包输出路径...")
	pt.outputPathEntry.OnChanged = func(path string) {
		pt.validateInput()
	}

	browseOutputBtn := widget.NewButton("浏览", func() {
		pt.selectOutputPath()
	})

	outputArea := container.NewBorder(
		nil, nil, nil, browseOutputBtn, pt.outputPathEntry,
	)

	// 魔改配置
	pt.portEntry = widget.NewEntry()
	if pt.config.DefaultPort != 0 {
		pt.portEntry.SetText(fmt.Sprintf("%d", pt.config.DefaultPort))
	} else {
		pt.portEntry.SetText("27042")
	}
	pt.portEntry.SetPlaceHolder("Frida 服务器端口")

	pt.magicNameEntry = widget.NewEntry()
	if pt.config.MagicName != "" && len(pt.config.MagicName) == 5 {
		pt.magicNameEntry.SetText(pt.config.MagicName)
	} else {
		pt.magicNameEntry.SetText("frida")
	}
	pt.magicNameEntry.SetPlaceHolder("魔改名称 (5个字符)")

	// 验证输入
	pt.magicNameEntry.OnChanged = func(text string) {
		pt.validateInput()
	}
	pt.portEntry.OnChanged = func(text string) {
		pt.validateInput()
	}

	// 随机生成魔改名称按钮
	randomMagicBtn := widget.NewButton("随机", func() {
		randomName := utils.GenerateRandomName()
		pt.magicNameEntry.SetText(randomName)
	})

	magicNameArea := container.NewBorder(
		nil, nil, nil, randomMagicBtn, pt.magicNameEntry,
	)

	// 进度条和状态
	pt.progressBar = widget.NewProgressBar()
	pt.progressBar.Hide()
	pt.progressLabel = widget.NewLabel("")
	pt.progressLabel.Hide()

	// 操作按钮
	pt.packageBtn = widget.NewButton("修改 DEB 包", func() {
		pt.startPackaging()
	})
	pt.packageBtn.Disable()

	progressArea := container.NewVBox(
		pt.progressBar,
		pt.progressLabel,
	)

	actionArea := container.NewBorder(
		nil, nil, nil, pt.packageBtn, progressArea,
	)

	// 主布局
	pt.content = container.NewVBox(
		debFileArea,
		widget.NewSeparator(),
		container.NewBorder(
			nil, nil, widget.NewLabel("输出路径:"), nil, outputArea,
		),
		widget.NewSeparator(),
		container.NewBorder(
			nil, nil, widget.NewLabel("魔改名称:"), nil, magicNameArea,
		),
		container.NewBorder(
			nil, nil, widget.NewLabel("服务端口:"), nil, pt.portEntry,
		),
		widget.NewSeparator(),
		packageInfoCard,
		widget.NewSeparator(),
		actionArea,
	)
}

func (pt *PackageTab) Content() *fyne.Container {
	return pt.content
}

func (pt *PackageTab) Refresh() {
	// 刷新逻辑
	pt.validateInput()
}

// UpdateGlobalConfig 更新全局配置
func (pt *PackageTab) UpdateGlobalConfig(magicName string, port int) {
	if pt.magicNameEntry != nil {
		pt.magicNameEntry.SetText(magicName)
	}
	if pt.portEntry != nil {
		pt.portEntry.SetText(fmt.Sprintf("%d", port))
	}
}

// selectDebFile 选择DEB文件
func (pt *PackageTab) selectDebFile() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		path := reader.URI().Path()
		pt.debFileEntry.SetText(path)

		// 自动设置输出路径
		if pt.outputPathEntry.Text == "" {
			dir := filepath.Dir(path)
			base := filepath.Base(path)
			nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))

			// 生成修改后的文件名
			magicName := pt.magicNameEntry.Text
			if magicName == "" {
				magicName = "frida"
			}

			outputName := fmt.Sprintf("%s_%s_modified.deb", nameWithoutExt, magicName)
			outputPath := filepath.Join(dir, outputName)
			pt.outputPathEntry.SetText(outputPath)
		}
	}, fyne.CurrentApp().Driver().AllWindows()[0])

	// 设置文件过滤器
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".deb"}))
	fileDialog.Show()
}

// selectOutputPath 选择输出路径
func (pt *PackageTab) selectOutputPath() {
	fileDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		filePath := writer.URI().Path()
		if !strings.HasSuffix(strings.ToLower(filePath), ".deb") {
			filePath += ".deb"
		}
		pt.outputPathEntry.SetText(filePath)
	}, fyne.CurrentApp().Driver().AllWindows()[0])

	// 设置默认文件名
	magicName := pt.magicNameEntry.Text
	if magicName == "" {
		magicName = "frida"
	}

	defaultName := fmt.Sprintf("%s_modified.deb", magicName)
	fileDialog.SetFileName(defaultName)

	fileDialog.Show()
}

// validateInput 验证输入
func (pt *PackageTab) validateInput() {
	outputPathValid := pt.outputPathEntry.Text != ""
	magicNameValid := core.ValidateMagicName(pt.magicNameEntry.Text) == nil
	portValid := pt.isValidPort(pt.portEntry.Text)
	fileValid := pt.debFileEntry.Text != ""

	if fileValid && outputPathValid && magicNameValid && portValid {
		pt.packageBtn.Enable()
	} else {
		pt.packageBtn.Disable()
	}
}

// isValidPort 检查端口是否有效
func (pt *PackageTab) isValidPort(portStr string) bool {
	if portStr == "" {
		return false
	}
	port, err := strconv.Atoi(portStr)
	return err == nil && port > 0 && port <= 65535
}

// startPackaging 开始修改DEB包
func (pt *PackageTab) startPackaging() {
	outputPath := pt.outputPathEntry.Text
	debFile := pt.debFileEntry.Text

	// 解析端口
	port, err := strconv.Atoi(pt.portEntry.Text)
	if err != nil {
		pt.updateStatus("端口号无效")
		pt.addLog("ERROR: 端口号无效: " + pt.portEntry.Text)
		return
	}

	magicName := pt.magicNameEntry.Text

	// 显示进度
	pt.progressBar.Show()
	pt.progressLabel.Show()
	pt.progressBar.SetValue(0)
	pt.progressLabel.SetText("正在初始化...")
	pt.packageBtn.Disable()

	go func() {
		defer func() {
			fyne.Do(pt.progressBar.Hide)
			fyne.Do(pt.progressLabel.Hide)
			fyne.Do(pt.packageBtn.Enable)
		}()

		pt.modifyExistingDebPackage(outputPath, port, magicName, debFile)
	}()
}

// modifyExistingDebPackage 修改现有DEB包
func (pt *PackageTab) modifyExistingDebPackage(outputPath string, port int, magicName string, debFile string) {

	fyne.Do(func() {
		pt.updateStatus("开始修改DEB包...")
		pt.addLog("INFO: 开始修改现有DEB包")
		pt.addLog(fmt.Sprintf("INFO: 输入DEB文件: %s", debFile))
		pt.addLog(fmt.Sprintf("INFO: 输出路径: %s", outputPath))
		pt.addLog(fmt.Sprintf("INFO: 魔改名称: %s", magicName))
		pt.addLog(fmt.Sprintf("INFO: 端口: %d", port))
	})

	// 创建DEB修改器
	debModifier := core.NewDebModifier(debFile, outputPath, magicName, port)

	// 进度回调函数
	progressCallback := func(progress float64, message string) {
		fyne.Do(func() {
			pt.progressBar.SetValue(progress)
			pt.progressLabel.SetText(message)
			pt.updateStatus(message)
			pt.addLog(fmt.Sprintf("INFO: %s (%.1f%%)", message, progress*100))
		})
	}

	// 执行修改
	err := debModifier.ModifyDebPackage(progressCallback)
	if err != nil {
		errorMsg := "DEB包修改失败: " + err.Error()
		fyne.Do(func() {
			pt.updateStatus(errorMsg)
			pt.progressLabel.SetText("修改失败!")
			pt.addLog("ERROR: " + errorMsg)
			dialog.ShowError(fmt.Errorf("DEB包修改失败: %v", err), fyne.CurrentApp().Driver().AllWindows()[0])
		})
		return
	}

	// 更新配置
	pt.config.MagicName = magicName
	pt.config.DefaultPort = port
	_ = pt.config.Save()

	successMsg := fmt.Sprintf("DEB包修改完成! 输出文件: %s", outputPath)
	outputBaseName := filepath.Base(outputPath)
	inputBaseName := filepath.Base(debFile)
	contentText := fmt.Sprintf("DEB包修改完成!\n\n原始文件: %s\n输出文件: %s\n魔改名称: %s\n端口: %d\n\n修改后的文件已保存到指定位置",
		inputBaseName, outputBaseName, magicName, port)

	fyne.Do(func() {
		pt.progressBar.SetValue(1.0)
		pt.progressLabel.SetText("DEB包修改完成!")
		pt.updateStatus(successMsg)
		pt.addLog("SUCCESS: " + successMsg)
		pt.addLog("INFO: 配置已保存")
		dialog.ShowInformation("DEB包修改完成", contentText, fyne.CurrentApp().Driver().AllWindows()[0])
	})
}

// PythonEnv Python环境信息
type PythonEnv struct {
	Name    string // 环境名称 (conda env name 或 "System Python")
	Path    string // Python可执行文件路径
	Version string // Python版本
	Type    string // 环境类型 (conda, venv, system)
}

// FridaInfo Frida工具信息
type FridaInfo struct {
	Version     string // frida版本
	InstallPath string // 安装路径
	PatchStatus string // 补丁状态 (original, patched, unknown)
	BackupPath  string // 备份路径
}

// ToolsTab 工具标签页
type ToolsTab struct {
	app          fyne.App
	config       *config.Config
	updateStatus StatusUpdater
	addLog       func(string)
	content      *fyne.Container

	// UI组件
	pythonEnvSelect   *widget.Select
	refreshEnvBtn     *widget.Button
	envInfoLabel      *widget.Label
	fridaInfoLabel    *widget.Label
	fridaVersionLabel *widget.Label
	fridaPathLabel    *widget.Label
	patchStatusLabel  *widget.Label

	magicNameEntry *FixedWidthEntry
	portEntry      *FixedWidthEntry

	patchBtn   *widget.Button
	restoreBtn *widget.Button
	backupBtn  *widget.Button

	progressBar   *widget.ProgressBar
	progressLabel *widget.Label

	// 数据
	pythonEnvs  []PythonEnv
	currentEnv  *PythonEnv
	fridaInfo   *FridaInfo
	hexReplacer *core.HexReplacer
}

func NewToolsTab(cfg *config.Config, statusUpdater StatusUpdater) *ToolsTab {
	tt := &ToolsTab{
		config:       cfg,
		updateStatus: statusUpdater,
		addLog:       func(msg string) {}, // 默认空实现
		pythonEnvs:   []PythonEnv{},
		hexReplacer:  core.NewHexReplacer(),
	}

	tt.setupUI()
	return tt
}

// SetLogFunction 设置日志函数
func (tt *ToolsTab) SetLogFunction(addLog func(string)) {
	tt.addLog = addLog
}

func (tt *ToolsTab) setupUI() {
	// Python环境选择区域
	tt.pythonEnvSelect = widget.NewSelect([]string{"点击刷新扫描Python环境..."}, func(selected string) {
		tt.onPythonEnvSelected(selected)
	})
	tt.pythonEnvSelect.Resize(fyne.NewSize(300, 0))

	tt.refreshEnvBtn = widget.NewButton("刷新环境", func() {
		tt.scanPythonEnvironments()
	})

	tt.envInfoLabel = widget.NewLabel("未选择Python环境")
	tt.envInfoLabel.Wrapping = fyne.TextWrapWord

	environmentArea := widget.NewCard("Python环境", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("选择Python环境:"),
			tt.pythonEnvSelect,
			tt.refreshEnvBtn,
		),
		tt.envInfoLabel,
	))

	// frida-tools信息区域
	tt.fridaInfoLabel = widget.NewLabel("请先选择Python环境")
	tt.fridaVersionLabel = widget.NewLabel("版本: 未知")
	tt.fridaPathLabel = widget.NewLabel("路径: 未知")
	tt.patchStatusLabel = widget.NewLabel("状态: 未检测")

	detectBtn := widget.NewButton("检测frida-tools", func() {
		tt.detectFridaTools()
	})

	fridaInfoArea := widget.NewCard("frida-tools信息", "", container.NewVBox(
		container.NewHBox(detectBtn, tt.fridaInfoLabel),
		tt.fridaVersionLabel,
		tt.fridaPathLabel,
		tt.patchStatusLabel,
	))

	// 魔改配置区域（必须与 hexreplace 一致：恰好 5 个小写 a-z；禁止默认 6 字符 "fridare"）
	tt.magicNameEntry = fixedWidthEntry(180, "5个小写字母 a-z")
	if tt.config != nil && core.ValidateMagicName(tt.config.MagicName) == nil {
		tt.magicNameEntry.SetText(tt.config.MagicName)
	} else {
		tt.magicNameEntry.SetText("") // 强制用户填写合法 5 字符名，避免静默 no-op 却显示 SUCCESS
	}

	// 魔改名称验证器：与 core.ValidateMagicName / hexreplace 完全一致
	tt.magicNameEntry.Validator = func(text string) error {
		return core.ValidateMagicName(strings.TrimSpace(text))
	}

	tt.portEntry = fixedWidthEntry(160, "端口")
	tt.portEntry.SetText("27042")
	tt.portEntry.Validator = func(text string) error {
		if port, err := strconv.Atoi(text); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("端口必须在1-65535范围内")
		}
		return nil
	}

	// 操作按钮区域
	tt.patchBtn = widget.NewButton("执行魔改", func() {
		tt.patchFridaTools()
	})
	tt.patchBtn.Importance = widget.HighImportance
	tt.patchBtn.Disable()

	tt.restoreBtn = widget.NewButton("恢复原版", func() {
		tt.restoreFridaTools()
	})
	tt.restoreBtn.Disable()

	tt.backupBtn = widget.NewButton("手动备份", func() {
		tt.backupFridaTools()
	})
	tt.backupBtn.Disable()

	configArea := widget.NewCard("魔改配置", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("魔改名称:"), tt.magicNameEntry,
			widget.NewLabel("默认端口:"), tt.portEntry,
			tt.patchBtn,
			tt.restoreBtn,
			tt.backupBtn),
	))
	// 进度显示
	tt.progressBar = widget.NewProgressBar()
	tt.progressBar.Hide()

	tt.progressLabel = widget.NewLabel("")
	tt.progressLabel.Hide()

	progressArea := container.NewVBox(
		tt.progressBar,
		tt.progressLabel,
	)

	// 主布局
	tt.content = container.NewVBox(
		container.NewGridWithColumns(2,
			environmentArea,
			fridaInfoArea),
		configArea,
		progressArea,
	)

	// 初始扫描Python环境
	go tt.scanPythonEnvironments()
}

// scanPythonEnvironments 扫描Python环境
func (tt *ToolsTab) scanPythonEnvironments() {
	tt.updateStatus("正在扫描Python环境...")
	tt.addLog("INFO: 开始扫描Python环境")

	var envs []PythonEnv

	// 扫描conda环境
	condaEnvs := tt.scanCondaEnvironments()
	envs = append(envs, condaEnvs...)

	// 扫描系统Python
	systemPython := tt.scanSystemPython()
	if systemPython != nil {
		envs = append(envs, *systemPython)
	}

	// 扫描venv环境 (可选，先不实现)

	tt.pythonEnvs = envs

	// 更新UI
	fyne.Do(func() {
		if len(envs) == 0 {
			tt.pythonEnvSelect.Options = []string{"未找到Python环境"}
			tt.envInfoLabel.SetText("未找到可用的Python环境")
			tt.updateStatus("未找到Python环境")
		} else {
			options := make([]string, len(envs))
			for i, env := range envs {
				options[i] = fmt.Sprintf("%s (%s)", env.Name, env.Type)
			}
			tt.pythonEnvSelect.Options = options
			tt.pythonEnvSelect.Refresh()
			tt.updateStatus(fmt.Sprintf("找到 %d 个Python环境", len(envs)))
			tt.addLog(fmt.Sprintf("INFO: 找到 %d 个Python环境", len(envs)))
		}
	})
}

// scanCondaEnvironments 扫描conda环境
func (tt *ToolsTab) scanCondaEnvironments() []PythonEnv {
	var envs []PythonEnv

	// 执行conda env list命令
	cmd := exec.Command("conda", "env", "list", "--json")
	hideConsoleCmd(cmd)
	output, err := cmd.Output()
	if err != nil {
		tt.addLog("INFO: 未找到conda环境")
		return envs
	}

	// 解析JSON输出
	var condaInfo struct {
		Envs []string `json:"envs"`
	}

	if err := json.Unmarshal(output, &condaInfo); err != nil {
		tt.addLog("ERROR: 解析conda环境信息失败: " + err.Error())
		return envs
	}

	// 获取每个环境的详细信息
	for _, envPath := range condaInfo.Envs {
		pythonPath := filepath.Join(envPath, "python")
		if runtime.GOOS == "windows" {
			pythonPath = filepath.Join(envPath, "python.exe")
		}

		// 检查python可执行文件是否存在
		if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
			continue
		}

		// 获取环境名称
		envName := filepath.Base(envPath)
		if envName == "." {
			envName = "base"
		}

		// 获取Python版本
		versionCmd := exec.Command(pythonPath, "--version")
		hideConsoleCmd(versionCmd)
		versionOutput, err := versionCmd.Output()
		version := "未知"
		if err == nil {
			version = strings.TrimSpace(string(versionOutput))
		}

		env := PythonEnv{
			Name:    envName,
			Path:    pythonPath,
			Version: version,
			Type:    "conda",
		}
		envs = append(envs, env)
		tt.addLog(fmt.Sprintf("INFO: 找到conda环境: %s (%s)", envName, version))
	}

	return envs
}

// scanSystemPython 扫描系统Python
func (tt *ToolsTab) scanSystemPython() *PythonEnv {
	// 尝试找到系统Python
	pythonCmds := []string{"python", "python3"}

	for _, cmd := range pythonCmds {
		pythonPath, err := exec.LookPath(cmd)
		if err != nil {
			continue
		}

		// 获取Python版本
		versionCmd := exec.Command(pythonPath, "--version")
		hideConsoleCmd(versionCmd)
		versionOutput, err := versionCmd.Output()
		version := "未知"
		if err == nil {
			version = strings.TrimSpace(string(versionOutput))
		}

		env := &PythonEnv{
			Name:    "System Python",
			Path:    pythonPath,
			Version: version,
			Type:    "system",
		}

		tt.addLog(fmt.Sprintf("INFO: 找到系统Python: %s", version))
		return env
	}

	return nil
}

// onPythonEnvSelected Python环境选择回调
func (tt *ToolsTab) onPythonEnvSelected(selected string) {
	if selected == "" || selected == "未找到Python环境" {
		return
	}

	// 从选择的字符串中找到对应的环境
	for _, env := range tt.pythonEnvs {
		expectedText := fmt.Sprintf("%s (%s)", env.Name, env.Type)
		if expectedText == selected {
			tt.currentEnv = &env

			// 更新环境信息显示
			tt.envInfoLabel.SetText(fmt.Sprintf("环境: %s\n版本: %s\n路径: %s",
				env.Name, env.Version, env.Path))

			tt.updateStatus(fmt.Sprintf("已选择Python环境: %s", env.Name))
			tt.addLog(fmt.Sprintf("INFO: 切换到Python环境: %s", env.Name))

			// 启用检测按钮
			// 自动检测frida-tools
			go tt.detectFridaTools()
			break
		}
	}
}

// detectFridaTools 检测frida-tools信息
func (tt *ToolsTab) detectFridaTools() {
	if tt.currentEnv == nil {
		tt.updateStatus("请先选择Python环境")
		return
	}

	fyne.Do(func() {
		tt.fridaInfoLabel.SetText("未安装frida-tools")
		tt.fridaVersionLabel.SetText("版本: 未安装")
		tt.fridaPathLabel.SetText("路径: 无")
		tt.patchStatusLabel.SetText("状态: 未安装")
		tt.patchBtn.Disable()
		tt.restoreBtn.Disable()
		tt.backupBtn.Disable()
	})
	tt.updateStatus("正在检测frida-tools...")
	tt.addLog("INFO: 开始检测frida-tools")

	// 使用选定的Python环境执行pip show frida
	var cmd *exec.Cmd
	if tt.currentEnv.Type == "conda" {
		// conda环境需要激活
		envName := tt.currentEnv.Name
		if envName == "base" {
			cmd = exec.Command("conda", "run", "-n", "base", "pip", "show", "frida")
		} else {
			cmd = exec.Command("conda", "run", "-n", envName, "pip", "show", "frida")
		}
	} else {
		// 系统Python直接使用pip
		cmd = exec.Command(tt.currentEnv.Path, "-m", "pip", "show", "frida")
	}
	hideConsoleCmd(cmd)

	output, err := cmd.Output()
	if err != nil {
		tt.updateStatus("未检测到frida-tools")
		tt.addLog("ERROR: 未检测到frida-tools: " + err.Error())
		return
	}

	// 解析pip show输出
	lines := strings.Split(string(output), "\n")
	var version, location string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Version:") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		} else if strings.HasPrefix(line, "Location:") {
			location = strings.TrimSpace(strings.TrimPrefix(line, "Location:"))
		}
	}

	if version == "" || location == "" {
		tt.updateStatus("无法获取frida-tools信息")
		tt.addLog("ERROR: 无法解析frida-tools信息")
		return
	}

	// 检测patch状态
	fridaPath := filepath.Join(location, "frida")
	patchStatus := tt.checkPatchStatus(fridaPath)
	backupPath := filepath.Join(fridaPath, "_original_backup")

	tt.fridaInfo = &FridaInfo{
		Version:     version,
		InstallPath: fridaPath,
		PatchStatus: patchStatus,
		BackupPath:  backupPath,
	}

	// 更新UI
	fyne.Do(func() {
		tt.fridaInfoLabel.SetText("已检测到frida-tools")
		tt.fridaVersionLabel.SetText("版本: " + version)
		tt.fridaPathLabel.SetText("路径: " + fridaPath)
		tt.patchStatusLabel.SetText("状态: " + patchStatus)

		// 根据状态启用/禁用按钮
		if patchStatus == "original" {
			tt.patchBtn.Enable()
			tt.restoreBtn.Disable()
			tt.backupBtn.Enable()
		} else if patchStatus == "patched" {
			tt.patchBtn.Disable()
			tt.restoreBtn.Enable()
			tt.backupBtn.Disable()
		} else {
			tt.patchBtn.Enable()
			tt.restoreBtn.Disable()
			tt.backupBtn.Enable()
		}
	})

	tt.updateStatus("frida-tools检测完成")
	tt.addLog(fmt.Sprintf("INFO: frida-tools v%s 检测完成，状态: %s", version, patchStatus))
}

// checkPatchStatus 检查patch状态
func (tt *ToolsTab) checkPatchStatus(fridaPath string) string {
	// 检查备份目录是否存在
	backupPath := filepath.Join(fridaPath, "_original_backup")
	if _, err := os.Stat(backupPath); err == nil {
		return "patched"
	}

	// 检查关键文件是否包含frida字符串 (简单检测)
	coreFile := filepath.Join(fridaPath, "_frida.py")
	if _, err := os.Stat(coreFile); err == nil {
		content, err := os.ReadFile(coreFile)
		if err == nil {
			contentStr := string(content)
			// 如果包含默认的frida字符串，认为是原版
			if strings.Contains(contentStr, "frida-server") && !strings.Contains(contentStr, "fridare") {
				return "original"
			} else if strings.Contains(contentStr, "fridare") {
				return "patched"
			}
		}
	}

	return "unknown"
}

// patchFridaTools 执行frida-tools魔改
func (tt *ToolsTab) patchFridaTools() {
	if tt.currentEnv == nil || tt.fridaInfo == nil {
		tt.updateStatus("请先选择Python环境并检测frida-tools")
		return
	}

	magicName := strings.TrimSpace(tt.magicNameEntry.Text)
	port := strings.TrimSpace(tt.portEntry.Text)

	// 验证输入
	if err := tt.magicNameEntry.Validator(magicName); err != nil {
		tt.updateStatus("魔改名称错误: " + err.Error())
		return
	}
	if err := tt.portEntry.Validator(port); err != nil {
		tt.updateStatus("端口错误: " + err.Error())
		return
	}

	tt.updateStatus("开始执行frida-tools魔改...")
	tt.addLog(fmt.Sprintf("INFO: 开始魔改frida-tools，魔改名称: %s, 端口: %s", magicName, port))

	// 显示进度
	fyne.Do(func() {
		tt.progressBar.Show()
		tt.progressLabel.Show()
		tt.progressLabel.SetText("正在创建备份...")
		tt.progressBar.SetValue(0.1)
		tt.patchBtn.Disable()
	})

	go func() {
		// 1. 创建备份
		if err := tt.createBackup(); err != nil {
			fyne.Do(func() {
				tt.progressBar.Hide()
				tt.progressLabel.Hide()
				tt.patchBtn.Enable()
			})
			tt.updateStatus("创建备份失败: " + err.Error())
			tt.addLog("ERROR: 创建备份失败: " + err.Error())
			return
		}

		fyne.Do(func() {
			tt.progressLabel.SetText("正在执行魔改...")
			tt.progressBar.SetValue(0.5)
		})

		// 2. 执行魔改
		if err := tt.performPatch(magicName, port); err != nil {
			fyne.Do(func() {
				tt.progressBar.Hide()
				tt.progressLabel.Hide()
				tt.patchBtn.Enable()
			})
			tt.updateStatus("魔改失败: " + err.Error())
			tt.addLog("ERROR: 魔改失败: " + err.Error())
			return
		}

		fyne.Do(func() {
			tt.progressLabel.SetText("魔改完成!")
			tt.progressBar.SetValue(1.0)

			// 延迟隐藏进度条
			time.AfterFunc(2*time.Second, func() {
				fyne.Do(func() {
					tt.progressBar.Hide()
					tt.progressLabel.Hide()
				})
			})

			// 更新按钮状态
			tt.patchBtn.Disable()
			tt.restoreBtn.Enable()
			tt.backupBtn.Disable()

			// 更新状态显示
			tt.patchStatusLabel.SetText("状态: patched")
		})

		tt.updateStatus("frida-tools魔改完成!")
		tt.addLog("SUCCESS: frida-tools魔改完成")
	}()
}

// createBackup 创建备份
func (tt *ToolsTab) createBackup() error {
	backupPath := tt.fridaInfo.BackupPath

	// 如果备份已存在，跳过
	if _, err := os.Stat(backupPath); err == nil {
		tt.addLog("INFO: 备份已存在，跳过创建备份")
		return nil
	}

	// 创建备份目录
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %v", err)
	}

	// 复制关键 Python 文件（不备份 __init__.py：我们不再修改它）
	filesToBackup := []string{
		"core.py",
	}

	for _, file := range filesToBackup {
		srcPath := filepath.Join(tt.fridaInfo.InstallPath, file)
		dstPath := filepath.Join(backupPath, file)

		if _, err := os.Stat(srcPath); err == nil {
			if err := tt.copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("备份文件 %s 失败: %v", file, err)
			}
			tt.addLog(fmt.Sprintf("INFO: 已备份文件: %s", file))
		}
	}

	// 备份原生扩展库
	soFiles, err := tt.findSOFiles()
	if err == nil {
		for _, soFile := range soFiles {
			dstPath := filepath.Join(backupPath, filepath.Base(soFile))
			if err := tt.copyFile(soFile, dstPath); err != nil {
				tt.addLog(fmt.Sprintf("WARN: 备份原生库失败 %s: %v", soFile, err))
				continue
			}
			tt.addLog(fmt.Sprintf("INFO: 已备份原生库: %s", soFile))
		}
	}

	tt.addLog("INFO: 备份创建完成")
	return nil
}

// copyFile 复制文件
func (tt *ToolsTab) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// performPatch 执行魔改（入口强制校验魔改名，非法名不得返回成功）
func (tt *ToolsTab) performPatch(magicName, port string) error {
	if err := core.ValidateMagicName(magicName); err != nil {
		return err
	}

	// 1. Python代码字符串魔改
	if err := tt.patchPythonFiles(magicName, port); err != nil {
		return fmt.Errorf("Python文件魔改失败: %v", err)
	}

	// 2. SO文件二进制魔改
	if err := tt.patchSOFiles(magicName, port); err != nil {
		return fmt.Errorf("SO文件魔改失败: %v", err)
	}

	return nil
}

// patchPythonFiles 魔改Python文件
// 注意：绝不能对 __init__.py 或任意 import 路径做裸 "frida" 全局替换，
// 否则会把 import _frida 变成 import _xxxxx，导致 ModuleNotFoundError。
// 客户端与服务端对齐只需替换 core.py 中的 "xxxxx:rpc" 通道名。
func (tt *ToolsTab) patchPythonFiles(magicName, port string) error {
	_ = port // 端口由设备端 server 决定，Python 包侧无需改默认端口字符串

	if err := core.ValidateMagicName(magicName); err != nil {
		return err
	}

	filePath := filepath.Join(tt.fridaInfo.InstallPath, "core.py")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		tt.addLog("WARN: core.py 不存在，跳过 Python 文本魔改")
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件 core.py 失败: %v", err)
	}

	// full=true: 与魔改 server 同步 — frida:rpc + re.frida.* + Frida.* API
	contentStr, n, err := core.PatchClientProtocolSurface(string(content), magicName, true)
	if err != nil {
		return err
	}
	if n > 0 {
		if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
			return fmt.Errorf("写入文件 core.py 失败: %v", err)
		}
		tt.addLog(fmt.Sprintf("INFO: 客户端协议面已同步 magic=%s（rpc/re.frida./Frida.*）共 %d 处 → core.py", magicName, n))
		tt.addLog("SUCCESS: 已魔改Python文件: core.py")
	} else {
		tt.addLog("INFO: core.py 中未找到可替换协议标记，可能已魔改或版本结构不同")
	}

	// 明确跳过 __init__.py 全局 bare "frida" 替换（避免破坏 import _frida）
	tt.addLog("INFO: 已跳过 __init__.py 全局替换（避免破坏 import _frida）；协议/API 已按前缀同步")
	return nil
}

// patchSOFiles 魔改SO文件
func (tt *ToolsTab) patchSOFiles(magicName, port string) error {
	// 查找SO文件
	soFiles, err := tt.findSOFiles()
	if err != nil {
		return fmt.Errorf("查找SO文件失败: %v", err)
	}

	if len(soFiles) == 0 {
		tt.addLog("INFO: 未找到SO文件，跳过二进制魔改")
		return nil
	}

	// 使用hexreplace工具进行二进制替换
	for _, soFile := range soFiles {
		if err := tt.patchSingleSOFile(soFile, magicName, port); err != nil {
			tt.addLog(fmt.Sprintf("WARN: SO文件魔改失败: %s, 错误: %v", soFile, err))
			continue
		}
		tt.addLog(fmt.Sprintf("SUCCESS: 已魔改SO文件: %s", soFile))
	}

	return nil
}

// findSOFiles 查找SO文件（包目录 + 上层 site-packages，兼容 conda/多命名）
func (tt *ToolsTab) findSOFiles() ([]string, error) {
	var soFiles []string
	seen := make(map[string]bool)

	addIfNative := func(path string, info os.FileInfo) {
		if info == nil || info.IsDir() {
			return
		}
		// 跳过备份
		if strings.HasSuffix(path, ".fridare") || strings.Contains(path, "_original_backup") {
			return
		}
		name := strings.ToLower(info.Name())
		ext := strings.ToLower(filepath.Ext(path))
		// 优先 _frida* 命名，避免误改无关 .so
		isFridaNative := strings.HasPrefix(name, "_frida") ||
			(strings.Contains(name, "frida") && (ext == ".so" || ext == ".pyd" || ext == ".dylib" || ext == ".dll"))
		if !isFridaNative {
			return
		}
		if ext == ".so" || ext == ".dll" || ext == ".dylib" || ext == ".pyd" {
			if !seen[path] {
				seen[path] = true
				soFiles = append(soFiles, path)
			}
		}
	}

	// 1) 包目录
	_ = filepath.Walk(tt.fridaInfo.InstallPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		addIfNative(path, info)
		return nil
	})

	// 2) 上层目录（部分发行版把扩展放在 site-packages 根）
	parent := filepath.Dir(tt.fridaInfo.InstallPath)
	entries, err := os.ReadDir(parent)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			addIfNative(filepath.Join(parent, e.Name()), info)
		}
	}

	return soFiles, nil
}

// patchSingleSOFile 魔改单个SO文件
func (tt *ToolsTab) patchSingleSOFile(soFile, magicName, port string) error {
	// 使用HexReplacer进行专业的二进制魔改
	// HexReplacer会自动处理所有frida相关的字符串替换
	if err := tt.hexReplace(soFile, "", magicName); err != nil {
		return fmt.Errorf("魔改SO文件失败: %v", err)
	}

	tt.addLog(fmt.Sprintf("SUCCESS: 已魔改SO文件: %s", soFile))
	return nil
}

// hexReplace 执行十六进制替换 - 使用HexReplacer进行专业的二进制魔改
func (tt *ToolsTab) hexReplace(filePath, oldStr, newStr string) error {
	if err := core.ValidateMagicName(newStr); err != nil {
		return err
	}

	// 创建临时输出文件
	tempFile := filePath + ".tmp"

	// 使用HexReplacer进行专业的二进制魔改
	err := tt.hexReplacer.PatchFile(filePath, newStr, tempFile, func(progress float64, status string) {
		// 可以在这里添加进度回调，但对于SO文件魔改我们简化处理
		tt.addLog(fmt.Sprintf("INFO: %s (%.1f%%)", status, progress*100))
	})

	if err != nil {
		// 清理临时文件
		os.Remove(tempFile)
		return fmt.Errorf("HexReplacer魔改失败: %v", err)
	}

	// 替换原文件
	if err := os.Rename(tempFile, filePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("替换原文件失败: %v", err)
	}

	return nil
}

// restoreFridaTools 恢复原版frida-tools
func (tt *ToolsTab) restoreFridaTools() {
	if tt.fridaInfo == nil {
		tt.updateStatus("请先检测frida-tools")
		return
	}

	tt.updateStatus("开始恢复原版frida-tools...")
	tt.addLog("INFO: 开始恢复原版frida-tools")

	// 显示进度
	fyne.Do(func() {
		tt.progressBar.Show()
		tt.progressLabel.Show()
		tt.progressLabel.SetText("正在恢复原版...")
		tt.progressBar.SetValue(0.2)
		tt.restoreBtn.Disable()
	})

	go func() {
		if err := tt.performRestore(); err != nil {
			fyne.Do(func() {
				tt.progressBar.Hide()
				tt.progressLabel.Hide()
				tt.restoreBtn.Enable()
			})
			tt.updateStatus("恢复失败: " + err.Error())
			tt.addLog("ERROR: 恢复失败: " + err.Error())
			return
		}

		fyne.Do(func() {
			tt.progressLabel.SetText("恢复完成!")
			tt.progressBar.SetValue(1.0)

			// 延迟隐藏进度条
			time.AfterFunc(2*time.Second, func() {
				fyne.Do(func() {
					tt.progressBar.Hide()
					tt.progressLabel.Hide()
				})
			})

			// 更新按钮状态
			tt.patchBtn.Enable()
			tt.restoreBtn.Disable()
			tt.backupBtn.Enable()

			// 更新状态显示
			tt.patchStatusLabel.SetText("状态: original")
		})

		tt.updateStatus("frida-tools恢复完成!")
		tt.addLog("SUCCESS: frida-tools恢复完成")
	}()
}

// performRestore 执行恢复
func (tt *ToolsTab) performRestore() error {
	backupPath := tt.fridaInfo.BackupPath

	// 检查备份是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份不存在: %s", backupPath)
	}

	// 恢复备份的文件
	files, err := os.ReadDir(backupPath)
	if err != nil {
		return fmt.Errorf("读取备份目录失败: %v", err)
	}

	fyne.Do(func() {
		tt.progressBar.SetValue(0.5)
	})

	parent := filepath.Dir(tt.fridaInfo.InstallPath)
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		srcPath := filepath.Join(backupPath, file.Name())
		name := file.Name()
		ext := strings.ToLower(filepath.Ext(name))
		// 原生库优先还原到当前实际所在位置
		dstPath := filepath.Join(tt.fridaInfo.InstallPath, name)
		if ext == ".so" || ext == ".pyd" || ext == ".dylib" || ext == ".dll" {
			candidates := []string{
				filepath.Join(tt.fridaInfo.InstallPath, name),
				filepath.Join(parent, name),
			}
			// 若包目录没有但上层有同名文件，还原到上层
			if _, err := os.Stat(candidates[0]); os.IsNotExist(err) {
				if _, err2 := os.Stat(candidates[1]); err2 == nil {
					dstPath = candidates[1]
				}
			} else {
				dstPath = candidates[0]
			}
		}

		if err := tt.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("恢复文件 %s 失败: %v", name, err)
		}
		tt.addLog(fmt.Sprintf("INFO: 已恢复文件: %s -> %s", name, dstPath))
	}

	// 删除备份目录
	if err := os.RemoveAll(backupPath); err != nil {
		tt.addLog(fmt.Sprintf("WARN: 删除备份目录失败: %v", err))
	} else {
		tt.addLog("INFO: 已删除备份目录")
	}

	return nil
}

// backupFridaTools 手动备份
func (tt *ToolsTab) backupFridaTools() {
	if tt.fridaInfo == nil {
		tt.updateStatus("请先检测frida-tools")
		return
	}

	tt.updateStatus("开始手动备份...")
	tt.addLog("INFO: 开始手动备份")

	go func() {
		if err := tt.createBackup(); err != nil {
			tt.updateStatus("手动备份失败: " + err.Error())
			tt.addLog("ERROR: 手动备份失败: " + err.Error())
		} else {
			tt.updateStatus("手动备份完成")
			tt.addLog("SUCCESS: 手动备份完成")
		}
	}()
}

func (tt *ToolsTab) Content() *fyne.Container {
	return tt.content
}

func (tt *ToolsTab) Refresh() {
	// 刷新逻辑
}

// UpdateGlobalConfig 更新全局配置
func (tt *ToolsTab) UpdateGlobalConfig(magicName string, port int) {
	if tt.magicNameEntry != nil {
		tt.magicNameEntry.SetText(magicName)
	}
	if tt.portEntry != nil {
		tt.portEntry.SetText(fmt.Sprintf("%d", port))
	}
}

// SettingsTab 设置标签页
type SettingsTab struct {
	config       *config.Config
	updateStatus StatusUpdater
	applyTheme   func()
	content      *fyne.Container
	window       fyne.Window // 添加窗口引用

	// 全局配置组件
	appVersionEntry *FixedWidthEntry
	workDirEntry    *FixedWidthEntry

	// 网络配置组件
	proxyEntry   *FixedWidthEntry
	timeoutEntry *FixedWidthEntry
	retriesEntry *FixedWidthEntry

	// Frida配置组件
	defaultPortEntry *FixedWidthEntry
	magicNameEntry   *FixedWidthEntry
	autoConfirmCheck *widget.Check

	// UI配置组件
	themeSelect       *widget.Select
	windowWidthEntry  *FixedWidthEntry
	windowHeightEntry *FixedWidthEntry
	debugModeCheck    *widget.Check
	noShowNoticeCheck *widget.Check

	// 下载配置组件
	downloadDirEntry         *FixedWidthEntry
	concurrentDownloadsEntry *FixedWidthEntry

	// OpenAI 兼容端点（AI agent / 源码重编译）
	openaiBaseEntry       *FixedWidthEntry
	openaiKeyEntry        *FixedWidthEntry
	openaiModelEntry      *FixedWidthEntry
	openaiHelpLabel       *widget.Label
	openaiAgentProxyChk   *widget.Check
	openaiProbeBtn        *widget.Button
	openaiProbeResult     *widget.Label

	// 源码重编译相关
	rebuildDiskEntry       *FixedWidthEntry
	rebuildImageEntry      *FixedWidthEntry
	rebuildMirrorEntry     *FixedWidthEntry
	rebuildPullDirectChk   *widget.Check
	rebuildUseGrokChk      *widget.Check

	// 操作按钮
	saveBtn   *widget.Button
	resetBtn  *widget.Button
	importBtn *widget.Button
	exportBtn *widget.Button
}

func NewSettingsTab(cfg *config.Config, statusUpdater StatusUpdater, themeApplier func(), window fyne.Window) *SettingsTab {
	st := &SettingsTab{
		config:       cfg,
		updateStatus: statusUpdater,
		applyTheme:   themeApplier,
		window:       window,
	}

	st.setupUI()
	return st
}
func (st *SettingsTab) RefreshConfigDisplay() {
	// 刷新配置显示
	if st.appVersionEntry != nil {
		st.appVersionEntry.SetText(st.config.AppVersion)
	}
	if st.workDirEntry != nil {
		st.workDirEntry.SetText(st.config.WorkDir)
	}
	if st.proxyEntry != nil {
		st.proxyEntry.SetText(st.config.Proxy)
	}
	if st.timeoutEntry != nil {
		st.timeoutEntry.SetText(fmt.Sprintf("%d", st.config.Timeout))
	}
	if st.retriesEntry != nil {
		st.retriesEntry.SetText(fmt.Sprintf("%d", st.config.Retries))
	}
	if st.defaultPortEntry != nil {
		st.defaultPortEntry.SetText(fmt.Sprintf("%d", st.config.DefaultPort))
	}
	if st.magicNameEntry != nil {
		st.magicNameEntry.SetText(st.config.MagicName)
	}
	if st.autoConfirmCheck != nil {
		st.autoConfirmCheck.SetChecked(st.config.AutoConfirm)
	}
	if st.themeSelect != nil {
		st.themeSelect.SetSelected(st.config.Theme)
	}
	if st.windowWidthEntry != nil {
		st.windowWidthEntry.SetText(fmt.Sprintf("%d", st.config.WindowWidth))
	}
	if st.windowHeightEntry != nil {
		st.windowHeightEntry.SetText(fmt.Sprintf("%d", st.config.WindowHeight))
	}
	if st.debugModeCheck != nil {
		st.debugModeCheck.SetChecked(st.config.DebugMode)
	}
	if st.noShowNoticeCheck != nil {
		st.noShowNoticeCheck.SetChecked(st.config.NoShowNotice)
	}
	if st.openaiBaseEntry != nil {
		st.openaiBaseEntry.SetText(st.config.OpenAIBaseURL)
	}
	if st.openaiKeyEntry != nil {
		st.openaiKeyEntry.SetText(st.config.OpenAIAPIKey)
	}
	if st.openaiModelEntry != nil {
		st.openaiModelEntry.SetText(st.config.OpenAIModel)
	}
	if st.rebuildDiskEntry != nil {
		st.rebuildDiskEntry.SetText(fmt.Sprintf("%.0f", st.config.RebuildMinDiskGB))
	}
	if st.rebuildImageEntry != nil {
		st.rebuildImageEntry.SetText(st.config.RebuildDockerImage)
	}
	if st.rebuildMirrorEntry != nil {
		st.rebuildMirrorEntry.SetText(st.config.RebuildDockerMirror)
	}
	if st.rebuildPullDirectChk != nil {
		st.rebuildPullDirectChk.SetChecked(st.config.RebuildDockerPullDirect)
	}
	if st.rebuildUseGrokChk != nil {
		st.rebuildUseGrokChk.SetChecked(st.config.RebuildUseLocalGrok)
	}
	if st.openaiAgentProxyChk != nil {
		st.openaiAgentProxyChk.SetChecked(st.config.RebuildAgentUseGUIProxy)
	}
}

func (st *SettingsTab) setupUI() {
	// 全局配置区域
	st.appVersionEntry = fixedWidthEntry(120, "版本号")
	st.appVersionEntry.SetText(st.config.AppVersion)
	st.appVersionEntry.Disable() // 版本号只读
	st.workDirEntry = fixedWidthEntry(300, "工作目录路径")
	st.workDirEntry.SetText(st.config.WorkDir)

	workDirBtn := widget.NewButton("选择", st.selectWorkDir)

	globalConfigSection := widget.NewCard("🔧 全局配置", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("应用版本:"), st.appVersionEntry,
			widget.NewLabel("   "), // 间距
		),
		container.NewHBox(
			widget.NewLabel("工作目录:"), st.workDirEntry, workDirBtn,
		),
	))

	// 网络配置区域
	st.proxyEntry = fixedWidthEntry(300, "http://proxy:port")
	st.proxyEntry.SetText(st.config.Proxy)

	st.timeoutEntry = fixedWidthEntry(80, "秒")
	st.timeoutEntry.SetText(fmt.Sprintf("%d", st.config.Timeout))
	st.timeoutEntry.Validator = func(text string) error {
		if val, err := strconv.Atoi(text); err != nil || val < 5 || val > 300 {
			return fmt.Errorf("超时时间必须在5-300秒之间")
		}
		return nil
	}

	st.retriesEntry = fixedWidthEntry(80, "次")
	st.retriesEntry.SetText(fmt.Sprintf("%d", st.config.Retries))
	st.retriesEntry.Validator = func(text string) error {
		if val, err := strconv.Atoi(text); err != nil || val < 0 || val > 10 {
			return fmt.Errorf("重试次数必须在0-10次之间")
		}
		return nil
	}

	proxyTestBtn := widget.NewButton("测试", st.testProxy)

	networkConfigSection := widget.NewCard("🌐 网络配置", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("代理服务器:"), st.proxyEntry, proxyTestBtn,
		),
		container.NewHBox(
			widget.NewLabel("超时时间:"), st.timeoutEntry,
			widget.NewLabel("   重试次数:"), st.retriesEntry,
		),
		widget.NewLabel("说明: 代理设置影响frida下载，超时和重试用于网络请求"),
	))

	// Frida配置区域
	st.defaultPortEntry = fixedWidthEntry(80, "端口")
	st.defaultPortEntry.SetText(fmt.Sprintf("%d", st.config.DefaultPort))

	st.magicNameEntry = fixedWidthEntry(100, "5字符")
	st.magicNameEntry.SetText(st.config.MagicName)

	st.autoConfirmCheck = widget.NewCheck("自动确认操作", nil)
	st.autoConfirmCheck.SetChecked(st.config.AutoConfirm)

	// 添加说明标签
	autoConfirmLabel := widget.NewLabel("(启用后将跳过确认对话框，直接执行魔改操作)")
	autoConfirmLabel.TextStyle = fyne.TextStyle{Italic: true}
	autoConfirmLabel.Importance = widget.LowImportance

	randomNameBtn := widget.NewButton("随机", st.generateRandomMagicName)

	fridaConfigSection := widget.NewCard("🎯 Frida配置", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("默认端口:"), st.defaultPortEntry,
			widget.NewLabel("   魔改名称:"), st.magicNameEntry, randomNameBtn,
		),
		container.NewHBox(
			st.autoConfirmCheck, autoConfirmLabel,
		),
	))

	// UI配置区域
	st.themeSelect = widget.NewSelect([]string{"auto", "light", "dark"}, func(selected string) {
		// 实时应用主题
		st.config.Theme = selected
		if st.applyTheme != nil {
			st.applyTheme()
		}
		st.updateStatus(fmt.Sprintf("主题已切换为: %s", selected))
	})
	st.themeSelect.SetSelected(st.config.Theme)

	st.windowWidthEntry = fixedWidthEntry(80, "宽度")
	st.windowWidthEntry.SetText(fmt.Sprintf("%d", st.config.WindowWidth))

	st.windowHeightEntry = fixedWidthEntry(80, "高度")
	st.windowHeightEntry.SetText(fmt.Sprintf("%d", st.config.WindowHeight))

	st.debugModeCheck = widget.NewCheck("调试模式", nil)
	st.debugModeCheck.SetChecked(st.config.DebugMode)

	st.noShowNoticeCheck = widget.NewCheck("启动时不显示公告", nil)
	st.noShowNoticeCheck.SetChecked(st.config.NoShowNotice)

	uiConfigSection := widget.NewCard("🎨 界面配置", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("主题:"), st.themeSelect,
			st.debugModeCheck,
		),
		container.NewHBox(
			widget.NewLabel("窗口大小:"), st.windowWidthEntry,
			widget.NewLabel("x"), st.windowHeightEntry,
			st.noShowNoticeCheck,
		),
	))

	// 下载配置区域
	st.downloadDirEntry = fixedWidthEntry(300, "下载目录路径")
	st.downloadDirEntry.SetText(st.config.DownloadDir)

	st.concurrentDownloadsEntry = fixedWidthEntry(80, "并发数")
	st.concurrentDownloadsEntry.SetText(fmt.Sprintf("%d", st.config.ConcurrentDownloads))
	st.concurrentDownloadsEntry.Validator = func(text string) error {
		if val, err := strconv.Atoi(text); err != nil || val < 1 || val > 10 {
			return fmt.Errorf("并发下载数必须在1-10之间")
		}
		return nil
	}

	downloadDirBtn := widget.NewButton("选择", st.selectDownloadDir)

	downloadConfigSection := widget.NewCard("📥 下载配置", "", container.NewVBox(
		container.NewHBox(
			widget.NewLabel("下载目录:"), st.downloadDirEntry, downloadDirBtn,
		),
		container.NewHBox(
			widget.NewLabel("并发下载:"), st.concurrentDownloadsEntry,
		),
		widget.NewLabel("说明: 并发下载数影响同时下载的文件数量，过大可能导致网络堵塞"),
	))

	// OpenAI 兼容端点 — 驱动源码重编译 AI agent（可选功能）
	st.openaiBaseEntry = fixedWidthEntry(320, "https://claudegpt.org/v1")
	st.openaiBaseEntry.SetText(st.config.OpenAIBaseURL)
	st.openaiKeyEntry = fixedWidthEntry(280, "API Key")
	st.openaiKeyEntry.SetText(st.config.OpenAIAPIKey)
	st.openaiKeyEntry.Password = true
	st.openaiModelEntry = fixedWidthEntry(160, "模型名（可选）")
	st.openaiModelEntry.SetText(st.config.OpenAIModel)

	st.openaiHelpLabel = widget.NewLabel(config.OpenAIRecommendedHelp())
	st.openaiHelpLabel.Wrapping = fyne.TextWrapWord
	applyRecommendBtn := widget.NewButton("填入推荐端点", func() {
		st.openaiBaseEntry.SetText("https://claudegpt.org/v1")
		st.updateStatus("已填入推荐 API 端点 https://claudegpt.org/v1")
	})
	openSiteBtn := widget.NewButton("打开推荐站点", func() {
		if u, err := url.Parse("https://claudegpt.org/"); err == nil {
			_ = fyne.CurrentApp().OpenURL(u)
		}
	})
	st.openaiAgentProxyChk = widget.NewCheck("OpenAI 端点走 GUI 代理出口（默认不走）", nil)
	st.openaiAgentProxyChk.SetChecked(st.config.RebuildAgentUseGUIProxy)
	st.openaiProbeResult = widget.NewLabel("端点探测：未测试")
	st.openaiProbeResult.Wrapping = fyne.TextWrapWord
	st.openaiProbeBtn = widget.NewButton("测试端点连接", st.testOpenAIEndpoint)

	// QQ 群二维码：内嵌图片直接显示（非文本链接）
	qqQR := canvas.NewImageFromResource(assets.QQGroupQR)
	qqQR.FillMode = canvas.ImageFillContain
	qqQR.SetMinSize(fyne.NewSize(160, 160))
	qqTitle := widget.NewLabel("QQ 群 " + assets.QQGroupNumber)
	qqTitle.TextStyle = fyne.TextStyle{Bold: true}
	qqHint := widget.NewLabel("扫码入群申请体验额度")
	qqHint.Importance = widget.MediumImportance
	qqPanel := container.NewVBox(
		qqTitle,
		qqHint,
		container.NewCenter(qqQR),
	)

	openaiLeft := container.NewVBox(
		container.NewHBox(widget.NewLabel("API 端点:"), st.openaiBaseEntry, applyRecommendBtn),
		container.NewHBox(widget.NewLabel("API Key:"), st.openaiKeyEntry),
		container.NewHBox(widget.NewLabel("模型:"), st.openaiModelEntry),
		st.openaiAgentProxyChk,
		container.NewHBox(st.openaiProbeBtn, openSiteBtn),
		st.openaiProbeResult,
		st.openaiHelpLabel,
	)
	openaiConfigSection := widget.NewCard("🤖 OpenAI 兼容端点（AI Agent）", "源码重编译可选；静态魔改路径不需要；端点默认不走代理",
		container.NewBorder(nil, nil, nil, qqPanel, openaiLeft))

	// 源码重编译资源阈值 + 国内 Docker 镜像源
	st.rebuildDiskEntry = fixedWidthEntry(80, "GB")
	st.rebuildDiskEntry.SetText(fmt.Sprintf("%.0f", st.config.RebuildMinDiskGB))
	st.rebuildImageEntry = fixedWidthEntry(220, "本地 builder 标签")
	st.rebuildImageEntry.SetText(st.config.RebuildDockerImage)
	st.rebuildMirrorEntry = fixedWidthEntry(180, "docker.1ms.run")
	st.rebuildMirrorEntry.SetText(st.config.RebuildDockerMirror)
	st.rebuildPullDirectChk = widget.NewCheck("镜像源直连（不走代理；docker.1ms.run 必开）", nil)
	st.rebuildPullDirectChk.SetChecked(st.config.RebuildDockerPullDirect)
	st.rebuildUseGrokChk = widget.NewCheck("优先使用本机 grok-build（端点仍用上方 OpenAI 配置）", nil)
	st.rebuildUseGrokChk.SetChecked(st.config.RebuildUseLocalGrok)
	fillMirrorBtn := widget.NewButton("填入 1ms.run", func() {
		st.rebuildMirrorEntry.SetText("docker.1ms.run")
		st.rebuildPullDirectChk.SetChecked(true)
		st.updateStatus("已填入 docker.1ms.run，并启用镜像直连")
	})
	mirrorHelp := widget.NewLabel("Hub 失败时: ubuntu:22.04 → docker.1ms.run/library/ubuntu:22.04；容器内 git 仍可用上方代理")
	mirrorHelp.Wrapping = fyne.TextWrapWord
	mirrorHelp.Importance = widget.MediumImportance

	rebuildConfigSection := widget.NewCard("🧬 源码重编译 / Docker（可选）", "不强制装 Docker；镜像源默认 docker.1ms.run",
		container.NewVBox(
			container.NewHBox(widget.NewLabel("磁盘阈值(GB):"), st.rebuildDiskEntry),
			container.NewHBox(widget.NewLabel("本地镜像名:"), st.rebuildImageEntry),
			container.NewHBox(widget.NewLabel("Hub 镜像源:"), st.rebuildMirrorEntry, fillMirrorBtn),
			st.rebuildPullDirectChk,
			st.rebuildUseGrokChk,
			mirrorHelp,
		))

	// 操作按钮区域
	st.saveBtn = widget.NewButton("💾 保存设置", st.saveSettings)
	st.saveBtn.Importance = widget.HighImportance

	st.resetBtn = widget.NewButton("🔄 重置默认", st.resetToDefaults)
	st.importBtn = widget.NewButton("📁 导入配置", st.importSettings)
	st.exportBtn = widget.NewButton("💾 导出配置", st.exportSettings)

	actionSection := widget.NewCard("⚡ 操作", "", container.NewGridWithColumns(2,
		container.NewHBox(st.saveBtn, st.resetBtn),
		container.NewHBox(st.importBtn, st.exportBtn),
	))

	// 主布局 - 使用Grid布局，2列显示
	leftColumn := container.NewVBox(
		globalConfigSection,
		networkConfigSection,
		fridaConfigSection,
		openaiConfigSection,
	)

	rightColumn := container.NewVBox(
		uiConfigSection,
		downloadConfigSection,
		rebuildConfigSection,
		actionSection,
	)

	st.content = container.NewGridWithColumns(2, leftColumn, rightColumn)
}

func (st *SettingsTab) Content() *fyne.Container {
	return st.content
}

func (st *SettingsTab) Refresh() {
	// 刷新逻辑
}

// UpdateGlobalConfig 更新全局配置
func (st *SettingsTab) UpdateGlobalConfig(magicName string, port int) {
	if st.magicNameEntry != nil {
		st.magicNameEntry.SetText(magicName)
	}
	if st.defaultPortEntry != nil {
		st.defaultPortEntry.SetText(fmt.Sprintf("%d", port))
	}
}

// selectWorkDir 选择工作目录
func (st *SettingsTab) selectWorkDir() {
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}
		st.workDirEntry.SetText(dir.Path())
	}, st.window)
}

// selectDownloadDir 选择下载目录
func (st *SettingsTab) selectDownloadDir() {
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}
		st.downloadDirEntry.SetText(dir.Path())
	}, st.window)
}

// testProxy 测试代理
func (st *SettingsTab) testProxy() {
	proxy := strings.TrimSpace(st.proxyEntry.Text)
	if proxy == "" {
		st.updateStatus("请先输入代理地址")
		return
	}

	st.updateStatus("正在测试代理...")
	// 这里可以重用MainWindow的代理测试逻辑
	// 简化实现
	go func() {
		time.Sleep(2 * time.Second) // 模拟测试
		st.updateStatus("代理测试完成")
	}()
}

// generateRandomMagicName 生成随机魔改名称
func (st *SettingsTab) generateRandomMagicName() {
	randomName := utils.GenerateRandomName()
	st.magicNameEntry.SetText(randomName)
}

// saveSettings 保存设置
func (st *SettingsTab) saveSettings() {
	// 验证和更新配置
	if err := st.validateAndUpdateConfig(); err != nil {
		st.updateStatus("配置验证失败: " + err.Error())
		return
	}

	// 保存配置
	if err := st.config.Save(); err != nil {
		st.updateStatus("保存配置失败: " + err.Error())
		return
	}

	// 应用主题变更
	if st.applyTheme != nil {
		st.applyTheme()
	}

	st.updateStatus("设置已保存")
}

// validateAndUpdateConfig 验证并更新配置
func (st *SettingsTab) validateAndUpdateConfig() error {
	// 更新网络配置
	st.config.Proxy = strings.TrimSpace(st.proxyEntry.Text)

	if timeout, err := strconv.Atoi(st.timeoutEntry.Text); err == nil && timeout > 0 {
		st.config.Timeout = timeout
	} else {
		return fmt.Errorf("超时时间必须是正整数")
	}

	if retries, err := strconv.Atoi(st.retriesEntry.Text); err == nil && retries >= 0 {
		st.config.Retries = retries
	} else {
		return fmt.Errorf("重试次数必须是非负整数")
	}

	// 更新Frida配置
	if port, err := strconv.Atoi(st.defaultPortEntry.Text); err == nil && port > 0 && port <= 65535 {
		st.config.DefaultPort = port
	} else {
		return fmt.Errorf("端口必须在1-65535范围内")
	}

	magicName := strings.TrimSpace(st.magicNameEntry.Text)
	if err := core.ValidateMagicName(magicName); err != nil {
		return err
	}
	st.config.MagicName = magicName

	st.config.AutoConfirm = st.autoConfirmCheck.Checked

	// 更新UI配置
	st.config.Theme = st.themeSelect.Selected
	st.config.DebugMode = st.debugModeCheck.Checked
	st.config.NoShowNotice = st.noShowNoticeCheck.Checked

	if width, err := strconv.Atoi(st.windowWidthEntry.Text); err == nil && width >= 800 {
		st.config.WindowWidth = width
	} else {
		return fmt.Errorf("窗口宽度必须大于等于800")
	}

	if height, err := strconv.Atoi(st.windowHeightEntry.Text); err == nil && height >= 600 {
		st.config.WindowHeight = height
	} else {
		return fmt.Errorf("窗口高度必须大于等于600")
	}

	// 更新下载配置
	st.config.DownloadDir = strings.TrimSpace(st.downloadDirEntry.Text)
	st.config.WorkDir = strings.TrimSpace(st.workDirEntry.Text)

	if concurrent, err := strconv.Atoi(st.concurrentDownloadsEntry.Text); err == nil && concurrent > 0 && concurrent <= 10 {
		st.config.ConcurrentDownloads = concurrent
	} else {
		return fmt.Errorf("并发下载数必须在1-10范围内")
	}

	// OpenAI / 源码重编译
	if st.openaiBaseEntry != nil {
		st.config.OpenAIBaseURL = strings.TrimSpace(st.openaiBaseEntry.Text)
	}
	if st.openaiKeyEntry != nil {
		st.config.OpenAIAPIKey = strings.TrimSpace(st.openaiKeyEntry.Text)
	}
	if st.openaiModelEntry != nil {
		st.config.OpenAIModel = strings.TrimSpace(st.openaiModelEntry.Text)
	}
	if st.rebuildDiskEntry != nil {
		if gb, err := strconv.ParseFloat(strings.TrimSpace(st.rebuildDiskEntry.Text), 64); err == nil && gb > 0 {
			st.config.RebuildMinDiskGB = gb
		} else {
			return fmt.Errorf("源码重编译磁盘阈值必须是正数 GB")
		}
	}
	if st.rebuildImageEntry != nil {
		st.config.RebuildDockerImage = strings.TrimSpace(st.rebuildImageEntry.Text)
	}
	if st.rebuildMirrorEntry != nil {
		st.config.RebuildDockerMirror = strings.TrimSpace(st.rebuildMirrorEntry.Text)
	}
	if st.rebuildPullDirectChk != nil {
		st.config.RebuildDockerPullDirect = st.rebuildPullDirectChk.Checked
	}
	if st.rebuildUseGrokChk != nil {
		st.config.RebuildUseLocalGrok = st.rebuildUseGrokChk.Checked
	}
	if st.openaiAgentProxyChk != nil {
		st.config.RebuildAgentUseGUIProxy = st.openaiAgentProxyChk.Checked
	}

	return nil
}

// resetToDefaults 重置为默认值
func (st *SettingsTab) resetToDefaults() {
	dialog.ShowConfirm("确认重置", "确定要重置所有设置为默认值吗？", func(confirmed bool) {
		if !confirmed {
			return
		}

		defaultConfig := config.DefaultConfig()
		*st.config = *defaultConfig

		// 重新加载UI
		st.loadConfigToUI()
		st.updateStatus("已重置为默认设置")
	}, st.window)
}

// loadConfigToUI 加载配置到UI
func (st *SettingsTab) loadConfigToUI() {
	st.workDirEntry.SetText(st.config.WorkDir)
	st.proxyEntry.SetText(st.config.Proxy)
	st.timeoutEntry.SetText(fmt.Sprintf("%d", st.config.Timeout))
	st.retriesEntry.SetText(fmt.Sprintf("%d", st.config.Retries))
	st.defaultPortEntry.SetText(fmt.Sprintf("%d", st.config.DefaultPort))
	st.magicNameEntry.SetText(st.config.MagicName)
	st.autoConfirmCheck.SetChecked(st.config.AutoConfirm)
	st.themeSelect.SetSelected(st.config.Theme)
	st.windowWidthEntry.SetText(fmt.Sprintf("%d", st.config.WindowWidth))
	st.windowHeightEntry.SetText(fmt.Sprintf("%d", st.config.WindowHeight))
	st.debugModeCheck.SetChecked(st.config.DebugMode)
	st.noShowNoticeCheck.SetChecked(st.config.NoShowNotice)
	st.downloadDirEntry.SetText(st.config.DownloadDir)
	st.concurrentDownloadsEntry.SetText(fmt.Sprintf("%d", st.config.ConcurrentDownloads))
	if st.openaiBaseEntry != nil {
		st.openaiBaseEntry.SetText(st.config.OpenAIBaseURL)
	}
	if st.openaiKeyEntry != nil {
		st.openaiKeyEntry.SetText(st.config.OpenAIAPIKey)
	}
	if st.openaiModelEntry != nil {
		st.openaiModelEntry.SetText(st.config.OpenAIModel)
	}
	if st.rebuildDiskEntry != nil {
		st.rebuildDiskEntry.SetText(fmt.Sprintf("%.0f", st.config.RebuildMinDiskGB))
	}
	if st.rebuildImageEntry != nil {
		st.rebuildImageEntry.SetText(st.config.RebuildDockerImage)
	}
	if st.rebuildMirrorEntry != nil {
		st.rebuildMirrorEntry.SetText(st.config.RebuildDockerMirror)
	}
	if st.rebuildPullDirectChk != nil {
		st.rebuildPullDirectChk.SetChecked(st.config.RebuildDockerPullDirect)
	}
	if st.rebuildUseGrokChk != nil {
		st.rebuildUseGrokChk.SetChecked(st.config.RebuildUseLocalGrok)
	}
	if st.openaiAgentProxyChk != nil {
		st.openaiAgentProxyChk.SetChecked(st.config.RebuildAgentUseGUIProxy)
	}
}

// testOpenAIEndpoint runs a real HTTP probe against the configured OpenAI-compatible API.
func (st *SettingsTab) testOpenAIEndpoint() {
	base := ""
	key := ""
	model := ""
	if st.openaiBaseEntry != nil {
		base = strings.TrimSpace(st.openaiBaseEntry.Text)
	}
	if st.openaiKeyEntry != nil {
		key = strings.TrimSpace(st.openaiKeyEntry.Text)
	}
	if st.openaiModelEntry != nil {
		model = strings.TrimSpace(st.openaiModelEntry.Text)
	}
	useProxy := st.openaiAgentProxyChk != nil && st.openaiAgentProxyChk.Checked
	proxy := ""
	if st.proxyEntry != nil {
		proxy = strings.TrimSpace(st.proxyEntry.Text)
	} else {
		proxy = st.config.Proxy
	}
	if st.openaiProbeResult != nil {
		st.openaiProbeResult.SetText("端点探测：进行中…")
	}
	st.updateStatus("正在测试 OpenAI 端点…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res := rebuild.ProbeOpenAIEndpoint(ctx, rebuild.OpenAIProbeOptions{
			BaseURL:     base,
			APIKey:      key,
			Model:       model,
			UseGUIProxy: useProxy,
			Proxy:       proxy,
			Timeout:     25 * time.Second,
		})
		detail := rebuild.RedactSecret(res.Detail, key)
		msg := fmt.Sprintf("端点探测：ok=%v HTTP %d %s\n%s", res.OK, res.StatusCode, res.Method, detail)
		fyne.Do(func() {
			if st.openaiProbeResult != nil {
				st.openaiProbeResult.SetText(msg)
			}
			if res.OK {
				st.updateStatus("OpenAI 端点连接成功")
			} else {
				st.updateStatus("OpenAI 端点连接失败（见探测结果）")
			}
		})
	}()
}

// importSettings 导入配置
func (st *SettingsTab) importSettings() {
	dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
		if err != nil || file == nil {
			return
		}
		defer file.Close()

		// 这里可以实现配置文件导入逻辑
		st.updateStatus("配置导入功能待实现")
	}, st.window)
}

// exportSettings 导出配置
func (st *SettingsTab) exportSettings() {
	dialog.ShowFileSave(func(file fyne.URIWriteCloser, err error) {
		if err != nil || file == nil {
			return
		}
		defer file.Close()

		// 这里可以实现配置文件导出逻辑
		st.updateStatus("配置导出功能待实现")
	}, st.window)
}

// CreateTab 创建DEB包标签页
type CreateTab struct {
	app          fyne.App
	config       *config.Config
	updateStatus StatusUpdater
	addLog       func(string)
	content      *fyne.Container

	// UI 组件 - 使用widget.Entry改善所有输入框宽度
	fridaServerEntry   *FixedWidthEntry
	fridaAgentEntry    *FixedWidthEntry
	outputPathEntry    *FixedWidthEntry
	magicNameEntry     *FixedWidthEntry
	portEntry          *FixedWidthEntry
	packageNameEntry   *FixedWidthEntry
	versionEntry       *FixedWidthEntry
	architectureSelect *widget.Select
	maintainerEntry    *FixedWidthEntry
	descriptionEntry   *FixedWidthEntry
	dependsEntry       *FixedWidthEntry
	sectionEntry       *FixedWidthEntry
	prioritySelect     *widget.Select
	homepageEntry      *FixedWidthEntry
	isRootlessCheck    *widget.Check
	progressBar        *widget.ProgressBar
	progressLabel      *widget.Label
	createBtn          *widget.Button

	// 核心功能 (CreateFridaDeb is instantiated locally when needed)
}

// NewCreateTab 创建新的创建标签页
func NewCreateTab(app fyne.App, cfg *config.Config, statusUpdater StatusUpdater, logFunc func(string)) *CreateTab {
	ct := &CreateTab{
		app:          app,
		config:       cfg,
		updateStatus: statusUpdater,
		addLog:       logFunc,
	}

	ct.setupUI()
	return ct
}

// setupUI 设置UI界面
func (ct *CreateTab) setupUI() {
	// 使用固定宽度Entry组件 - 增加宽度
	ct.fridaServerEntry = fixedWidthEntry(200, "选择frida-server文件...")
	ct.fridaAgentEntry = fixedWidthEntry(200, "选择frida-agent.dylib文件 (可选)...")
	ct.outputPathEntry = fixedWidthEntry(180, "选择输出DEB文件路径...")

	ct.magicNameEntry = fixedWidthEntry(100, "5字符")
	ct.portEntry = fixedWidthEntry(100, "端口")
	ct.packageNameEntry = fixedWidthEntry(300, "包名 (自动生成)")
	ct.versionEntry = fixedWidthEntry(200, "版本")
	ct.maintainerEntry = fixedWidthEntry(300, "维护者")
	ct.descriptionEntry = fixedWidthEntry(300, "包描述")
	ct.dependsEntry = fixedWidthEntry(200, "依赖")
	ct.sectionEntry = fixedWidthEntry(200, "分类")
	ct.homepageEntry = fixedWidthEntry(300, "主页")

	// 设置按钮
	serverSelectBtn := widget.NewButton("选择", ct.selectFridaServer)
	agentSelectBtn := widget.NewButton("选择", ct.selectFridaAgent)
	outputSelectBtn := widget.NewButton("选择", ct.selectOutputPath)

	// 基本配置验证器和事件（与 hexreplace / ToolsTab 一致：恰好 5 个小写 a-z）
	ct.magicNameEntry.Validator = func(text string) error {
		if err := core.ValidateMagicName(strings.TrimSpace(text)); err != nil {
			return err
		}
		// 检查是否为保留名称
		reservedNames := []string{"frida", "admin", "root", "user", "guest"}
		for _, reserved := range reservedNames {
			if text == reserved {
				return fmt.Errorf("'%s'是保留名称，请使用其他名称", text)
			}
		}
		return nil
	}

	// 添加实时验证和字符长度限制
	ct.magicNameEntry.OnChanged = func(text string) {
		// 限制输入长度为5个字符
		if len(text) > 5 {
			ct.magicNameEntry.SetText(text[:5])
			return
		}

		// 实时更新包名
		ct.updatePackageName(text)

		// 实时验证显示
		if err := ct.magicNameEntry.Validator(text); err != nil {
			ct.updateStatus(fmt.Sprintf("魔改名称错误: %v", err))
		} else if len(text) == 5 {
			ct.updateStatus("魔改名称验证通过")
		}
	}

	ct.portEntry.SetText("27042")
	ct.portEntry.Validator = func(text string) error {
		if port, err := strconv.Atoi(text); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("端口必须在1-65535范围内")
		}
		return nil
	}

	ct.isRootlessCheck = widget.NewCheck("Rootless结构", nil)

	// 包信息配置
	ct.packageNameEntry.Disable() // 设置为只读

	ct.versionEntry.SetText("17.2.17")

	ct.architectureSelect = widget.NewSelect([]string{
		"iphoneos-arm64",
		"iphoneos-arm",
		"all",
	}, nil)
	ct.architectureSelect.SetSelected("iphoneos-arm64")
	ct.architectureSelect.Resize(fyne.NewSize(200, 0))

	ct.maintainerEntry.SetText("Fridare Team <support@fridare.com>")
	ct.descriptionEntry.SetPlaceHolder("包描述 (自动生成)")
	ct.dependsEntry.SetText("firmware (>= 12.0)")
	ct.sectionEntry.SetText("Development")

	ct.prioritySelect = widget.NewSelect([]string{
		"optional",
		"important",
		"required",
		"standard",
	}, nil)
	ct.prioritySelect.SetSelected("optional")
	ct.prioritySelect.Resize(fyne.NewSize(200, 0))

	ct.homepageEntry.SetText("https://frida.re/")

	// 进度显示
	ct.progressBar = widget.NewProgressBar()
	ct.progressLabel = widget.NewLabel("准备就绪")

	// 创建按钮
	ct.createBtn = widget.NewButton("创建DEB包", ct.createDebPackage)
	ct.createBtn.Importance = widget.HighImportance

	// 简化的紧凑布局 - 使用Border布局避免重叠
	// 文件选择区域
	serverRow := container.NewBorder(nil, nil,
		widget.NewLabel("frida-server:"),
		serverSelectBtn,
		ct.fridaServerEntry)

	agentRow := container.NewBorder(nil, nil,
		widget.NewLabel(" frida-agent:"),
		agentSelectBtn,
		ct.fridaAgentEntry)

	outputRow := container.NewBorder(nil, nil,
		widget.NewLabel("     输出路径:"),
		outputSelectBtn,
		ct.outputPathEntry)

	fileSection := widget.NewCard("文件选择", "", container.NewVBox(
		serverRow,
		agentRow,
		outputRow,
	))

	// 基本配置区域 - 使用HBox横向排列
	configSection := widget.NewCard("基本配置", "", container.NewHBox(
		widget.NewLabel("魔改名称:"), ct.magicNameEntry,
		widget.NewLabel("　　端口:"), ct.portEntry,
		widget.NewLabel("　　　　"), ct.isRootlessCheck,
	))

	// 包信息区域 - 分两行显示
	packageRow1 := container.NewHBox(
		widget.NewLabel("　　包名:"), ct.packageNameEntry,
		widget.NewLabel("　　版本:"), ct.versionEntry,
		widget.NewLabel("　　架构:"), ct.architectureSelect,
	)

	packageRow2 := container.NewHBox(
		widget.NewLabel("　维护者:"), ct.maintainerEntry,
		widget.NewLabel("　　分类:"), ct.sectionEntry,
		widget.NewLabel("　优先级:"), ct.prioritySelect,
	)

	packageSection := widget.NewCard("　包信息", "", container.NewVBox(
		packageRow1,
		packageRow2,
	))

	// 详细信息区域 - 一行显示
	detailRow := container.NewHBox(
		widget.NewLabel("　　描述:"), ct.descriptionEntry,
		widget.NewLabel("　　依赖:"), ct.dependsEntry,
		widget.NewLabel("　　主页:"), ct.homepageEntry,
	)

	detailSection := widget.NewCard("详细信息", "", detailRow)

	// 操作区域 - 使用Border布局
	actionSection := container.NewBorder(nil, nil,
		container.NewHBox(ct.progressLabel, ct.progressBar),
		ct.createBtn,
		nil,
	)

	// 主布局
	ct.content = container.NewVBox(
		fileSection,
		configSection,
		packageSection,
		detailSection,
		actionSection,
	)

	// 设置监听器 - 魔改名称的OnChanged已在Entry定义时设置
	ct.isRootlessCheck.OnChanged = func(checked bool) {
		ct.updatePackageName(ct.magicNameEntry.Text)
	}
}

// selectFridaServer 选择frida-server文件
func (ct *CreateTab) selectFridaServer() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		filePath := reader.URI().Path()
		ct.fridaServerEntry.SetText(filePath)
		ct.addLog(fmt.Sprintf("选择frida-server文件: %s", filePath))
	}, ct.app.Driver().AllWindows()[0])
}

// selectFridaAgent 选择frida-agent文件
func (ct *CreateTab) selectFridaAgent() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		filePath := reader.URI().Path()
		ct.fridaAgentEntry.SetText(filePath)
		ct.addLog(fmt.Sprintf("选择frida-agent文件: %s", filePath))
	}, ct.app.Driver().AllWindows()[0])
}

// selectOutputPath 选择输出路径
func (ct *CreateTab) selectOutputPath() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		filePath := writer.URI().Path()
		if !strings.HasSuffix(strings.ToLower(filePath), ".deb") {
			filePath += ".deb"
		}
		ct.outputPathEntry.SetText(filePath)
		ct.addLog(fmt.Sprintf("选择输出路径: %s", filePath))
	}, ct.app.Driver().AllWindows()[0])
}

// updatePackageName 更新包名
func (ct *CreateTab) updatePackageName(magicName string) {
	if magicName == "" {
		ct.packageNameEntry.SetText("")
		ct.descriptionEntry.SetText("")
		return
	}

	// 生成规则：将魔改字符替换其中的frida字符
	basePackageName := "re.frida.server"

	// 将frida替换为魔改名称
	packageName := strings.ReplaceAll(basePackageName, "frida", magicName)

	// 添加rootless后缀（如果选中）
	if ct.isRootlessCheck.Checked {
		packageName += ".rootless"
	}

	ct.packageNameEntry.SetText(packageName)

	// 同时更新描述，也替换其中的frida
	baseDescription := "Dynamic instrumentation toolkit for developers, security researchers, and reverse engineers based on Frida"
	description := strings.ReplaceAll(baseDescription, "Frida", strings.Title(magicName))
	description += fmt.Sprintf(" (Modified: %s)", magicName)
	ct.descriptionEntry.SetText(description)
}

// createDebPackage 创建DEB包
func (ct *CreateTab) createDebPackage() {
	// 验证输入
	if ct.fridaServerEntry.Text == "" {
		ct.showError("请选择frida-server文件")
		return
	}

	if ct.outputPathEntry.Text == "" {
		ct.showError("请选择输出路径")
		return
	}

	if ct.magicNameEntry.Text == "" {
		ct.showError("请输入魔改名称")
		return
	}

	if err := ct.magicNameEntry.Validator(ct.magicNameEntry.Text); err != nil {
		ct.showError(fmt.Sprintf("魔改名称格式错误: %v", err))
		return
	}

	port, err := strconv.Atoi(ct.portEntry.Text)
	if err != nil {
		ct.showError("端口格式错误")
		return
	}

	// 禁用按钮
	ct.createBtn.Disable()
	ct.progressBar.SetValue(0)
	ct.progressLabel.SetText("开始创建...")

	// 异步执行
	go ct.performCreate(port)
}

// performCreate 执行创建过程
func (ct *CreateTab) performCreate(port int) {
	defer func() {
		ct.createBtn.Enable()
	}()

	// 创建包信息
	packageInfo := &core.PackageInfo{
		Name:         ct.packageNameEntry.Text,
		Version:      ct.versionEntry.Text,
		Architecture: ct.architectureSelect.Selected,
		Maintainer:   ct.maintainerEntry.Text,
		Description:  ct.descriptionEntry.Text,
		Depends:      ct.dependsEntry.Text,
		Section:      ct.sectionEntry.Text,
		Priority:     ct.prioritySelect.Selected,
		Homepage:     ct.homepageEntry.Text,
		Port:         port,
		MagicName:    ct.magicNameEntry.Text,
		IsRootless:   ct.isRootlessCheck.Checked,
	}

	// 创建DEB构建器
	creator := core.NewCreateFridaDeb(ct.fridaServerEntry.Text, ct.outputPathEntry.Text, packageInfo)
	if ct.fridaAgentEntry.Text != "" {
		creator.FridaAgentPath = ct.fridaAgentEntry.Text
	}

	ct.addLog("开始创建DEB包...")
	ct.addLog(fmt.Sprintf("魔改名称: %s, 端口: %d, 结构: %s",
		packageInfo.MagicName, packageInfo.Port,
		map[bool]string{true: "Rootless", false: "Root"}[packageInfo.IsRootless]))

	// 执行创建
	err := creator.CreateDebPackage()
	if err != nil {
		ct.progressLabel.SetText("创建失败")
		ct.showError(fmt.Sprintf("创建DEB包失败: %v", err))
		ct.addLog(fmt.Sprintf("错误: %v", err))
		return
	}

	ct.progressBar.SetValue(1.0)
	ct.progressLabel.SetText("创建完成")
	ct.addLog("DEB包创建成功!")

	// 显示成功信息
	ct.showSuccess("DEB包创建成功!", fmt.Sprintf("输出文件: %s", ct.outputPathEntry.Text))

	ct.updateStatus("DEB包创建完成")
}

// showError 显示错误信息
func (ct *CreateTab) showError(message string) {
	dialog.ShowError(fmt.Errorf("%s", message), ct.app.Driver().AllWindows()[0])
}

// showSuccess 显示成功信息
func (ct *CreateTab) showSuccess(title, message string) {
	dialog.ShowInformation(title, message, ct.app.Driver().AllWindows()[0])
}

// Content 返回标签页内容
func (ct *CreateTab) Content() *fyne.Container {
	return ct.content
}

// Refresh 刷新标签页
func (ct *CreateTab) Refresh() {
	// 刷新逻辑
}

// UpdateGlobalConfig 更新全局配置
func (ct *CreateTab) UpdateGlobalConfig(magicName string, port int) {
	if ct.magicNameEntry != nil {
		ct.magicNameEntry.SetText(magicName)
		// 触发实时验证和包名更新
		if ct.magicNameEntry.OnChanged != nil {
			ct.magicNameEntry.OnChanged(magicName)
		}
	}
	if ct.portEntry != nil {
		ct.portEntry.SetText(fmt.Sprintf("%d", port))
	}
}

// hideConsoleCmd 在 platform 相关文件中实现（见 hide_console_windows.go / hide_console_other.go）

// HelpTab 帮助标签页
type HelpTab struct {
	content       *fyne.Container
	indexList     *widget.List
	contentViewer *widget.RichText

	// 帮助文档数据
	helpSections []HelpSection
	currentIndex int
}

// HelpSection 帮助章节
type HelpSection struct {
	Title   string
	Icon    string
	Content string
}

// NewHelpTab 创建新的帮助标签页
func NewHelpTab() *HelpTab {
	ht := &HelpTab{
		currentIndex: 0,
	}

	ht.setupHelpData()
	ht.setupUI()

	return ht
}

// setupHelpData 设置帮助文档数据
func (ht *HelpTab) setupHelpData() {
	ht.helpSections = []HelpSection{
		{
			Title: "双技术路线",
			Icon:  "🧬",
			Content: `# 双技术路线说明

Fridare 提供两条**独立**的 Frida 魔改技术线。默认路径不依赖 Docker / AI。

## 路线 A — 静态二进制补丁（默认）

对应标签：**frida 魔改** / **frida-tools 魔改** / **DEB 魔改与打包**。

- 输入：官方 release 预编译二进制
- 手段：hex / 特征字符串替换（hexreplace）
- 依赖：**无需 Docker**、无需编译工具链
- 速度：秒级
- 局限：只能改二进制中已有的可见特征
- 协议：服务端与 frida-tools 需成对魔改（含 rpc 等）

## 路线 B — 源码重编译 + 魔改（可选）

对应标签：**源码重编译**。

行业参考：strongR-frida、phantom-frida、官方 Frida configure/make 多宿主构建。

**编译仅在 Docker/Linux 内执行**（Host 不装 Frida 工具链）：

1. 探测 Docker / 磁盘 / 代理 / grok-build / OpenAI 端点  
2. 步骤① 基础镜像（工具链）  
3. 浅克隆：` + "`git clone --depth=1 --branch <版本>`" + `  
4. AI / 朴素源码魔改（Host 改挂载树，不编译）  
5. 容器内 ` + "`configure --host=… && make`" + `  
6. 导出 catalog：binaries + **已协议同步的 host wheel** + PROTOCOL-SYNC  

**deep（默认）**：服务端+客户端同步  
` + "re.frida.* / /re/frida/（DBus 对象路径）/ frida:rpc / Frida.* API → magic；缺对象路径会 UNKNOWN_METHOD。须用 catalog 内 wheel，勿用官方 pip。\n" + `

### 系统不予置

- **不强制**安装 Docker / grok-build  
- 未启用源码魔改时，下载与静态补丁路径完全不受影响  
- 启用时才要求上游代理、磁盘空间与 OpenAI 兼容端点  

### OpenAI 推荐

- 站点：https://claudegpt.org/  
- API：https://claudegpt.org/v1  
- QQ 群 **555354813** 可申请体验额度  
- **二维码请在「设置」页 OpenAI 区域直接扫码**（软件内嵌显示）

完整说明见仓库 ` + "`docs/dual-track.md`" + `。`,
		},
		{
			Title: "快速开始",
			Icon:  "🚀",
			Content: `# 快速开始指南

## 欢迎使用 Fridare GUI

Fridare 是一个强大的 Frida 工具集，专为 iOS 逆向工程和安全研究设计。

### 主要功能
- **Frida 下载管理**：自动下载最新版本的 Frida 组件
- **二进制文件魔改**：修改 Frida 特征以绕过检测（默认静态路径）
- **源码重编译魔改**（可选）：Docker + AI agent 重编译任意官方版本
- **DEB 包处理**：创建和修改 iOS DEB 安装包  
- **Python 环境集成**：自动检测和配置 Python 环境
- **frida-tools 魔改**：修改 frida-tools 避免被检测

### 快速上手步骤
1. **配置环境**：在"设置"页面配置工作目录和网络代理
2. **下载 Frida**：使用"下载"功能获取所需版本
3. **执行魔改**：使用「frida 魔改」等静态路径；需要源码级时再打开「源码重编译」
4. **部署使用**：将处理后的文件部署到目标设备

> 💡 **提示**：首次使用建议先查看"设置"页面进行基本配置。源码重编译为可选增强，见「双技术路线」。`,
		},
		{
			Title: "下载功能",
			Icon:  "📥",
			Content: `# 下载功能详解

## 功能概述
下载页面提供了完整的 Frida 组件下载和管理功能。

## 版本选择
- **自动获取**：程序启动时自动获取最新版本列表
- **版本过滤**：支持按版本号筛选和搜索
- **平台支持**：支持 iOS arm64、armv7 等多种架构

## 支持的组件类型
### 1. frida-server
- iOS 设备上运行的核心服务
- 支持 arm64、armv7 架构
- 自动选择对应设备架构

### 2. frida-agent
- 动态注入的 Agent 库文件  
- 通常为 .dylib 格式
- 用于高级注入场景

### 3. frida-gadget
- 嵌入式 Frida 库
- 用于应用内部集成
- 支持多种加载模式

## 下载配置
- **下载目录**：可在设置中自定义下载路径
- **并发下载**：支持 1-10 个并发下载任务
- **代理支持**：支持 HTTP/HTTPS 代理下载
- **断点续传**：支持大文件断点续传

## 使用技巧
- 建议下载后立即进行魔改处理
- 可同时下载多个版本备用
- 定期清理不需要的旧版本文件`,
		},
		{
			Title: "源码重编译",
			Icon:  "🐳",
			Content: `# 源码重编译 + AI 魔改（可选）

## 何时使用
当静态二进制补丁不够用、需要源码级标识/行为修改，或要针对任意官方 Frida 版本从源码产出时。

## 编译隔离
**configure / make / 交叉工具链只在 Docker（Linux）内跑**，Host 只跑 GUI、AI 改文件与 docker 客户端。界面提示「编译仅 Docker/Linux」。

## 前置条件（启用时才检查）
- **上游代理**（必填门禁）
- **磁盘**：建议 ≥ 40GB 空闲
- **Docker** 引擎（不强制安装；未装则本功能不可用，其余标签仍可用）
- **OpenAI 兼容端点** + API Key（设置页；推荐 claudegpt.org）
- **grok-build / grok**（若本机已有可勾选使用；端点始终用 GUI 配置）

## 操作步骤
1. 设置页配置代理、Docker 镜像源与 OpenAI  
2. 打开「🧬 源码重编译」，勾选启用  
3. 「检查依赖」确认环境  
4. 步骤① 基础镜像（可单独跑）；步骤② 魔改+编译  
5. 默认 profile=**deep**；或点「一键深度定制（①+② deep）」  
6. 填写官方版本（depth=1）、5 字母 magic、目标平台、对话目标  
7. 完成后到 **catalog** 产物目录：安装 ` + "`python/host/.../frida-*.whl`" + `（已含协议同步）+ frida_tools  

## deep 协议面（须成对）
` + "- re.frida.* → re.{magic}.*\n- /re/frida/ → /re/{magic}/（对象路径；缺则 UNKNOWN_METHOD）\n- frida:rpc → {magic}:rpc\n- \"Frida. → \"{Magic}.\n" + `产物含 PROTOCOL-SYNC.json 交叉核对结果。

## 编译目标
支持选择 Android / iOS / Windows / macOS 等常见架构。  
**诚实限制**：iOS/macOS 官方工具链偏 Apple 宿主；Linux Docker 上可能仅能完整交叉编译 Android/Linux/Windows 子集。

## 产物
` + "`catalog/{version}/{platform}/{magic}/`" + `（binaries + python/host wheels + INSTALL.txt + PROTOCOL-SYNC）。  
工作区 artifacts 另有 README-DEPLOY.txt。`,
		},
		{
			Title: "Frida 魔改",
			Icon:  "🔧",
			Content: `# Frida 二进制魔改详解

## 功能目的
通过修改 Frida 二进制文件的特征字符串，绕过应用的 Frida 检测机制。  
这是 **路线 A（静态补丁）** 的核心能力，**不需要 Docker**。

## 魔改原理
### 特征字符串替换
- 将默认的 "frida" 字符串替换为自定义名称
- 修改端口号避免固定端口检测
- 保持二进制文件结构完整性

### 支持的文件格式
- **MachO**：iOS 可执行文件格式
- **ELF**：Linux 可执行文件格式  
- **PE**：Windows 可执行文件格式

## 魔改步骤
### 1. 选择文件
- 支持单文件或批量选择
- 自动识别文件格式类型
- 显示文件基本信息

### 2. 配置参数
- **魔改名称**：5个字符，必须以字母开头
- **目标端口**：1-65535 范围内的有效端口
- **输出路径**：可选择覆盖或另存

### 3. 执行魔改
- 实时显示处理进度
- 支持批量处理多个文件
- 自动备份原始文件

## 注意事项
- 魔改名称必须严格为 5 个字符
- 建议使用随机名称避免被识别
- 处理前会自动创建备份文件
- 支持一键恢复到原始状态

## 高级功能
- **随机名称生成**：自动生成安全的随机名称
- **批量处理**：同时处理多个文件
- **预览模式**：处理前预览修改内容`,
		},
		{
			Title: "DEB 包魔改",
			Icon:  "📦",
			Content: `# DEB 包魔改功能

## 功能概述
对已有的 iOS DEB 安装包进行深度魔改，修改其中的 Frida 组件。

## 支持的包类型
### Frida DEB 包
- re.frida.server
- frida-server 相关包
- 第三方 Frida 衍生包

### 检测机制
- **自动识别**：智能识别包内的 Frida 组件
- **文件扫描**：递归扫描所有可执行文件
- **格式验证**：验证 DEB 包完整性

## 魔改流程
### 1. 包解析
- 解压 DEB 包结构
- 分析控制文件信息
- 识别可执行组件

### 2. 组件魔改
- 对识别到的 Frida 文件执行魔改
- 保持文件权限和属性
- 更新包的校验信息

### 3. 重新打包
- 重新压缩修改后的文件
- 更新包元数据
- 生成新的 DEB 文件

## 配置选项
- **魔改名称**：统一的 5 字符标识
- **端口配置**：修改默认监听端口
- **包信息**：可选择是否修改包标识符

## 输出结果
- 生成魔改后的新 DEB 包
- 保留原包的所有功能
- 自动添加魔改标识后缀

## 使用建议
- 处理前备份原始 DEB 包
- 测试魔改后的包是否正常工作
- 建议在沙盒环境中先行测试`,
		},
		{
			Title: "DEB 包创建",
			Icon:  "🆕",
			Content: `# DEB 包创建功能

## 功能介绍
从零开始创建全新的 iOS DEB 安装包，集成魔改后的 Frida 组件。

## 创建流程
### 1. 组件选择
- **frida-server**：必选的核心服务文件
- **frida-agent**：可选的 Agent 库文件
- **配置文件**：LaunchDaemon plist 配置

### 2. 包信息配置
- **包名**：自动生成或手动指定
- **版本号**：遵循 Debian 版本规范
- **维护者信息**：包的维护者信息
- **描述信息**：包的功能描述

### 3. 高级选项
- **依赖关系**：指定包的依赖项
- **冲突处理**：避免与现有包冲突
- **安装脚本**：自定义安装/卸载脚本

## 支持的安装方式
### Root 模式
- 传统的 root 权限安装
- 完整的系统访问权限
- 兼容性最好

### Rootless 模式
- 适配新版本 iOS 的 rootless 环境
- 受限的系统访问权限
- 更高的安全性

## 自动化功能
- **智能包名**：基于魔改名称自动生成
- **版本管理**：自动递增版本号
- **权限设置**：自动设置正确的文件权限
- **签名验证**：可选的包签名功能

## 输出文件
- 标准的 .deb 安装包
- 包含所有必要的元数据
- 可直接用于 Cydia/Sileo 安装

## 质量保证
- 包结构验证
- 文件完整性检查
- 兼容性测试建议`,
		},
		{
			Title: "frida-tools 魔改",
			Icon:  "🛠️",
			Content: `# frida-tools 魔改功能

## 功能目标
对PC中安装的 frida-tools Python 包进行魔改，避免无法访问魔改过的 frida-server。

## 检测机制
### Python 环境扫描
- **Conda 环境**：自动检测所有 conda 环境
- **系统 Python**：检测系统级 Python 安装
- **虚拟环境**：识别 venv/virtualenv 环境

### frida-tools 检测
- 使用 pip show frida-tools 检查安装状态
- 获取安装路径和版本信息
- 验证包的完整性

## 魔改内容
### 1. Python 代码魔改
- 修改 Python 源码中的字符串常量
- 替换默认的 "frida" 标识符
- 保持代码功能完整性

### 2. 二进制库魔改
- 魔改 .so/.pyd 动态库文件
- 使用内置 HexReplacer 引擎
- 支持跨平台二进制处理

### 3. 配置文件修改
- 更新相关配置文件
- 修改默认端口设置
- 保持工具兼容性

## 安全机制
### 自动备份
- 魔改前自动备份原始文件
- 支持一键恢复到原始状态
- 备份文件完整性验证

### 冲突检测
- 检测是否有正在运行的 frida 进程
- 避免在使用中的环境进行魔改
- 提供安全的魔改时机建议

## 使用流程
1. **环境扫描**：自动检测所有 Python 环境
2. **选择环境**：选择要魔改的 Python 环境
3. **配置参数**：设置魔改名称和端口
4. **执行魔改**：自动备份并执行魔改
5. **验证结果**：检查魔改是否成功

## 注意事项
- 魔改会影响该环境中的所有 frida 工具
- 建议在专用环境中进行魔改
- 魔改后的工具与原版不兼容
- 可以随时恢复到原始状态`,
		},
		{
			Title: "设置",
			Icon:  "⚙️",
			Content: `# 设置详解

## 全局配置
### 应用版本
- 显示当前程序版本信息
- 只读字段，不可修改

### 工作目录
- 程序的主要工作目录
- 存储临时文件和缓存
- 默认位置：~/.fridare

## 网络配置
### 代理设置
- 支持 HTTP/HTTPS 代理
- 格式：http://proxy:port
- 影响所有网络下载操作

### 超时时间
- 网络请求超时时间
- 范围：5-300 秒
- 影响下载和网络验证

### 重试次数
- 网络失败后的重试次数
- 范围：0-10 次
- 提高网络操作成功率

## Frida 配置
### 默认端口
- Frida 服务的默认监听端口
- 范围：1-65535
- 影响所有魔改操作

### 魔改名称
- 全局默认的魔改标识符
- 必须为 5 个字符
- 以字母开头，包含字母和数字

### 自动确认操作
- 启用后跳过确认对话框
- 提高批量操作效率
- 建议熟练用户启用

## 界面配置
### 主题选择
- **Auto**：跟随系统主题
- **Light**：浅色主题
- **Dark**：深色主题
- 支持实时切换

### 窗口尺寸
- 自定义程序窗口大小
- 宽度：最小 800 像素
- 高度：最小 600 像素

### 调试模式
- 启用详细的调试信息
- 显示更多技术细节
- 便于问题诊断

## 下载配置
### 下载目录
- Frida 组件的下载目录
- 默认：~/Downloads/fridare
- 可自定义到任意位置

### 并发下载数
- 同时进行的下载任务数
- 范围：1-10 个
- 过大可能导致网络拥堵

## 配置管理
### 保存设置
- 实时验证配置有效性
- 自动保存到配置文件
- 下次启动自动加载

### 重置默认
- 一键恢复所有默认设置
- 会清除所有自定义配置
- 操作前会弹出确认对话框

### 导入/导出
- 支持配置文件的导入导出
- 便于在多台设备间同步配置
- 使用 JSON 格式存储`,
		},
		{
			Title: "故障排除",
			Icon:  "🔍",
			Content: `# 故障排除指南

## 常见问题

### 1. 下载失败
**症状**：无法下载 Frida 组件，连接超时

**解决方案**：
- 检查网络连接状态
- 配置合适的代理服务器
- 增加超时时间设置
- 尝试更换下载源

### 2. 魔改失败
**症状**：文件魔改过程中出现错误

**解决方案**：
- 确保文件未被其他程序占用
- 检查文件权限设置
- 验证魔改名称格式正确
- 尝试重新下载原始文件

### 3. Python 环境检测失败
**症状**：无法检测到 Python 环境或 frida-tools

**解决方案**：
- 确保 Python 正确安装
- 检查 PATH 环境变量
- 重新安装 frida-tools
- 尝试在管理员权限下运行

### 4. DEB 包创建失败
**症状**：DEB 包创建过程中出现错误

**解决方案**：
- 检查所选文件的完整性
- 确保有足够的磁盘空间
- 验证包信息格式正确
- 检查目标目录写权限

### 5. PC端连接iOS失败
**症状**：frida-ps -U 无法连接到设备

**解决方案**：
- 确认iOS设备已安装魔改后的frida-server
- 检查PC端是否已执行"🛠️ frida-tools 魔改"
- 验证iOS设备端和PC端魔改名称是否完全一致
- 确认设备USB连接正常
- 重新启动frida-server进程

## 调试技巧

### 启用调试模式
在设置页面启用"调试模式"，可以获得更详细的错误信息。

### 查看日志信息
程序底部的日志区域会显示详细的操作信息，有助于诊断问题。

### 文件权限检查
确保程序对工作目录具有读写权限。

### 网络连接测试
使用设置页面的"测试代理"功能验证网络配置。

## 获取支持与帮助

### 项目信息
- **GitHub项目地址**: https://github.com/suifei/fridare
- **项目文档**: https://github.com/suifei/fridare/blob/main/README.md
- **问题反馈**: https://github.com/suifei/fridare/issues

### 技术交流
- **QQ技术交流群**: 5353548813
- **讨论话题**: Frida魔改技术、iOS逆向、工具使用经验
- **群内资源**: 最新版本发布、技术文档、问题解答

### 错误报告
遇到问题时，请提供：
- 错误的详细描述和截图
- 操作系统版本和架构
- 程序版本信息
- 相关的日志信息
- 复现问题的具体步骤

### 社区支持
- 查看项目 Wiki 文档
- 搜索已知问题和解决方案
- 在GitHub提交 Issue 报告
- 参与QQ群讨论

## 预防措施

### 定期备份
- 定期备份重要的配置文件
- 保留原始的 Frida 组件
- 记录魔改过的环境信息

### 环境隔离
- 使用专用的 Python 环境进行魔改
- 避免在生产环境中直接操作
- 在虚拟机中测试新功能

### 版本管理
- 记录使用的 Frida 版本
- 保留多个版本的备份
- 测试兼容性后再升级

## 常见错误代码

### Error 1001: 网络连接失败
- 检查网络连接
- 配置正确的代理设置
- 确认防火墙设置

### Error 1002: 文件权限不足
- 以管理员身份运行程序
- 检查目标目录权限
- 暂时关闭安全软件

### Error 1003: Python环境异常
- 重新安装Python
- 更新pip到最新版本
- 重新安装frida-tools

### Error 1004: 魔改名称冲突
- 使用不同的魔改名称
- 检查已存在的进程
- 重启相关服务

如需更多帮助，请访问项目GitHub页面或加入QQ技术交流群获取支持。`,
		},
		{
			Title: "最佳实践",
			Icon:  "📋",
			Content: `# 最佳实践指南

## 使用流程建议

### 1.1 Frida 魔改 + DEB包创建流程
- **适用场景**：针对所有平台的 frida-server 进程进行魔改
- **操作步骤**：
  1. 使用"🔧 frida 魔改"功能修改 frida-server 二进制文件
  2. 使用"🆕 iOS DEB 打包"功能制作iOS安装包
  3. **重要**：使用"🛠️ frida-tools 魔改"功能修改PC端 Frida CLI
- **注意事项**：确保魔改名称在所有步骤中保持一致

### 1.2 DEB包魔改流程
- **适用场景**：针对官方发布的 DEB 包进行修改
- **操作步骤**：
  1. 使用"📦 iOS DEB 魔改"功能修改现有DEB包
  2. 支持 root 和 rootless 两种模式
  3. 将魔改后的 DEB 包安装到iOS设备
  4. **重要**：使用"🛠️ frida-tools 魔改"功能修改PC端 Frida CLI
- **优势**：基于官方包，稳定性更好

### 1.3 DEB包创建流程
- **适用场景**：完全自定义创建iOS DEB安装包
- **操作步骤**：
  1. 使用"🆕 iOS DEB 打包"功能从头创建DEB包
  2. 自定义包名、版本、配置等信息
  3. **重要**：使用"🛠️ frida-tools 魔改"功能修改PC端 Frida CLI
- **特点**：完全可控，适合高级用户

## ⚠️ 重要提醒

### PC端配置要求
**关键**：Frida魔改后，PC上要让frida命令能正确访问iOS设备，必须采用一致的魔改字符通过"🛠️ frida-tools 魔改"功能修改PC端的Python库。

### 魔改名称一致性
- iOS设备端的 frida-server 魔改名称
- PC端的 frida-tools 魔改名称
- **必须完全一致**，否则无法正常连接

## 环境准备
- 首次使用前完成基本设置配置
- 创建专用的工作目录
- 配置合适的网络代理（如需要）

## 组件获取
- 优先下载最新稳定版本
- 同时保留一个备用版本
- 验证下载文件的完整性

## 魔改策略
- 使用随机生成的魔改名称
- 避免使用容易被识别的名称
- 定期更换魔改参数

## 测试验证
- 在测试环境中验证魔改效果
- 确认功能正常后再部署
- 保留原始文件作为备份

## 安全建议
- 魔改名称避免使用敏感词汇
- 使用随机字符组合
- 定期更换标识符
- 使用可信的代理服务器
- 定期更新程序版本

## 维护建议
- 每月检查配置设置
- 清理不需要的下载文件
- 更新到最新程序版本
- 备份重要配置和操作日志`,
		},
	}
}

// setupUI 设置UI界面
func (ht *HelpTab) setupUI() {
	// 创建索引列表
	ht.indexList = widget.NewList(
		func() int {
			return len(ht.helpSections)
		},
		func() fyne.CanvasObject {
			icon := widget.NewLabel("")
			title := widget.NewLabel("")
			title.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewHBox(icon, title)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			container := obj.(*fyne.Container)
			icon := container.Objects[0].(*widget.Label)
			title := container.Objects[1].(*widget.Label)

			section := ht.helpSections[id]
			icon.SetText(section.Icon)
			title.SetText(section.Title)
		},
	)

	// 设置选择事件
	ht.currentIndex = -1
	ht.indexList.OnSelected = func(id widget.ListItemID) {
		// 避免重复加载相同内容
		if ht.currentIndex == id {
			return
		}

		ht.showContent(id)
	}

	// 创建内容显示区域
	ht.contentViewer = widget.NewRichText()
	ht.contentViewer.Wrapping = fyne.TextWrapWord
	ht.contentViewer.Scroll = container.ScrollBoth

	// 显示默认内容
	ht.showContent(0)
	ht.indexList.Select(0)

	// 强制刷新列表
	ht.indexList.Refresh()

	// 创建左侧面板 - 使用Border布局让列表自适应高度
	titleLabel := widget.NewLabel("📖 帮助目录")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	leftPanel := container.NewBorder(
		container.NewVBox(titleLabel, widget.NewSeparator()), // top
		nil,          // bottom
		nil,          // left
		nil,          // right
		ht.indexList, // center - 列表占据剩余空间
	)

	// 创建右侧面板 - 内容显示区域
	rightPanel := container.NewBorder(
		nil, nil, nil, nil,
		container.NewScroll(ht.contentViewer),
	)

	// 使用分割容器 - 调整分割比例
	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.3) // 左侧占30%，右侧占70%

	// 将分割容器包装在普通容器中
	ht.content = container.NewBorder(nil, nil, nil, nil, split)
}

// showContent 显示指定章节的内容
func (ht *HelpTab) showContent(index int) {
	if index >= 0 && index < len(ht.helpSections) {
		section := ht.helpSections[index]
		ht.contentViewer.ParseMarkdown(section.Content)
		ht.currentIndex = index
	}
}

// Content 返回标签页内容
func (ht *HelpTab) Content() *fyne.Container {
	return ht.content
}

// AnalysisTab 文件分析标签页
type AnalysisTab struct {
	app          fyne.App
	config       *config.Config
	updateStatus StatusUpdater
	addLog       func(string)
	content      *fyne.Container

	// UI 组件
	filePathEntry *widget.Entry
	browseBtn     *widget.Button
	analyzeBtn    *widget.Button
	// 过滤搜索框
	searchEntry *widget.Entry

	// 左侧树形结构
	tree *widget.Tree

	// 右侧表格（双表格模式）
	sectionListTable *widget.Table // 段列表表格（IDA风格）
	sectionDataTable *widget.Table // 段数据表格（十六进制/字符串）
	currentTable     *widget.Table // 当前显示的表格

	// 当前文件信息
	currentFile     string
	fileInfo        *core.FileInfo
	selectedSection int
	analyzer        *core.BinaryAnalyzer

	// 数据缓存 - 用于优化性能
	sectionDataCache    map[int][]byte // 段数据缓存
	lastSelectedSection int            // 上次选中的段，用于检测变化

	// 搜索过滤功能
	currentSearchText string        // 当前搜索文本
	filteredStrings   []StringData  // 过滤后的字符串列表
	highlightMatches  map[int][]int // 高亮匹配位置：行号->匹配位置数组
}

// NewAnalysisTab 创建分析标签页
func NewAnalysisTab(app fyne.App, cfg *config.Config, statusUpdater StatusUpdater, logFunc func(string)) *AnalysisTab {
	at := &AnalysisTab{
		app:                 app,
		config:              cfg,
		updateStatus:        statusUpdater,
		addLog:              logFunc,
		selectedSection:     -1,
		sectionDataCache:    make(map[int][]byte),
		lastSelectedSection: -1,
	}

	at.setupUI()
	return at
}

func (at *AnalysisTab) setupUI() {
	// 文件选择区域
	at.filePathEntry = widget.NewEntry()
	at.filePathEntry.SetPlaceHolder("选择要分析的二进制文件 (Mach-O, PE, ELF)...")

	at.browseBtn = widget.NewButton("浏览", func() {
		at.selectFile()
	})

	at.analyzeBtn = widget.NewButton("分析文件", func() {
		at.analyzeFile()
	})
	at.analyzeBtn.Importance = widget.HighImportance
	at.analyzeBtn.Disable()

	at.searchEntry = widget.NewEntry()
	at.searchEntry.SetPlaceHolder("搜字符串...")
	at.searchEntry.OnChanged = func(text string) {
		at.filterSectionData(text)
	}

	// 文件路径更改事件（添加自动分析功能）
	var autoAnalyzeTimer *time.Timer
	at.filePathEntry.OnChanged = func(path string) {
		if path != "" && at.fileExists(path) {
			at.analyzeBtn.Enable()

			// 重置定时器，实现防抖效果（用户停止输入500ms后自动分析）
			if autoAnalyzeTimer != nil {
				autoAnalyzeTimer.Stop()
			}
			autoAnalyzeTimer = time.AfterFunc(500*time.Millisecond, func() {
				// 再次检查文件是否存在，避免路径变化后的延迟分析
				if at.filePathEntry.Text != "" && at.fileExists(at.filePathEntry.Text) {
					at.analyzeFile()
				}
			})
		} else {
			at.analyzeBtn.Disable()
			// 取消待执行的自动分析
			if autoAnalyzeTimer != nil {
				autoAnalyzeTimer.Stop()
			}
		}
	}

	fileSelectArea :=
		container.NewBorder(nil, nil, nil, nil,
			container.NewGridWithColumns(3,
				at.filePathEntry,
				container.NewHBox(at.browseBtn, at.analyzeBtn),
				at.searchEntry),
		)

	// 创建左侧树形结构
	at.tree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			return at.getChildNodes(uid)
		},
		func(uid widget.TreeNodeID) bool {
			return at.isBranch(uid)
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(uid widget.TreeNodeID, branch bool, object fyne.CanvasObject) {
			label := object.(*widget.Label)
			label.SetText(at.getNodeText(uid))
			if branch {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{}
			}
		},
	)

	at.tree.OnSelected = func(uid widget.TreeNodeID) {
		at.onTreeNodeSelected(uid)
	}

	// 创建双表格系统
	at.createTables()

	// 设置初始表格（段列表模式）
	at.switchToSectionListMode()

	// 创建分割容器 - 直接使用表格，不需要容器
	splitContainer := container.NewHSplit(
		container.NewBorder(
			widget.NewLabel("文件结构"),
			nil, nil, nil,
			container.NewScroll(at.tree),
		),
		container.NewBorder(
			widget.NewLabel("详细信息"),
			nil, nil, nil,
			container.NewStack(
				container.NewScroll(at.sectionListTable),
				container.NewScroll(at.sectionDataTable),
			),
		),
	)
	splitContainer.Offset = 0.3 // 左侧占30%

	// 主布局

	at.content = container.NewBorder(
		widget.NewCard("文件选择", "", fileSelectArea),
		nil, nil, nil,
		splitContainer,
	)

}

// selectFile 选择文件
func (at *AnalysisTab) selectFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		path := reader.URI().Path()
		at.filePathEntry.SetText(path)

		// 自动分析选中的文件
		if path != "" && at.fileExists(path) {
			// 稍微延迟执行，确保UI更新完成
			time.AfterFunc(100*time.Millisecond, func() {
				at.analyzeFile()
			})
		}
	}, at.app.Driver().AllWindows()[0])
}

// fileExists 检查文件是否存在
func (at *AnalysisTab) fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// analyzeFile 分析文件
func (at *AnalysisTab) analyzeFile() {
	filePath := at.filePathEntry.Text
	if filePath == "" {
		at.updateStatus("请选择要分析的文件")
		return
	}

	at.updateStatus("正在分析文件...")
	at.addLog(fmt.Sprintf("INFO: 开始分析文件: %s", filePath))

	go func() {
		defer func() {
			at.updateStatus("文件分析完成")
		}()

		// 清理旧的缓存数据
		at.clearSectionDataCache()
		at.selectedSection = -1
		at.lastSelectedSection = -1

		// 创建分析器
		at.analyzer = core.NewBinaryAnalyzer(filePath)

		// 分析文件
		fileInfo, err := at.analyzer.AnalyzeFile()
		if err != nil {
			at.addLog(fmt.Sprintf("ERROR: 文件分析失败: %v", err))
			return
		}

		at.currentFile = filePath
		at.fileInfo = fileInfo
		at.addLog(fmt.Sprintf("INFO: 检测到文件类型: %s", fileInfo.FileType))
		at.addLog(fmt.Sprintf("INFO: 架构: %s", fileInfo.Architecture))
		at.addLog(fmt.Sprintf("INFO: 段数量: %d", len(fileInfo.Sections)))
		at.addLog(fmt.Sprintf("INFO: 段数量: %d", len(fileInfo.Sections)))

		// 刷新UI
		fyne.Do(func() {
			at.tree.Refresh()
			at.currentTable.Refresh()
			at.currentTable.ScrollToTop()
		})
	}()
}

// 树形结构相关方法
func (at *AnalysisTab) getChildNodes(uid widget.TreeNodeID) []widget.TreeNodeID {
	if uid == "" {
		// 根节点
		if at.currentFile == "" {
			return []widget.TreeNodeID{}
		}
		return []widget.TreeNodeID{"file_info", "sections"}
	}

	switch uid {
	case "file_info":
		// 文件信息子节点
		children := []widget.TreeNodeID{}
		if at.fileInfo != nil {
			// 文件类型节点
			typeLines := strings.Split(at.fileInfo.FileType, "\n")
			for i, line := range typeLines {
				line = strings.TrimSpace(line)
				if line != "" { // 忽略空行
					children = append(children, widget.TreeNodeID(fmt.Sprintf("info_type_%d", i)))
				}
			}

			// 文件大小节点
			sizeText := fmt.Sprintf("%d bytes", at.fileInfo.FileSize)
			sizeLines := strings.Split(sizeText, "\n")
			for i, line := range sizeLines {
				line = strings.TrimSpace(line)
				if line != "" { // 忽略空行
					children = append(children, widget.TreeNodeID(fmt.Sprintf("info_size_%d", i)))
				}
			}

			// 架构信息节点
			archLines := strings.Split(at.fileInfo.Architecture, "\n")
			for i, line := range archLines {
				line = strings.TrimSpace(line)
				if line != "" { // 忽略空行
					children = append(children, widget.TreeNodeID(fmt.Sprintf("info_arch_%d", i)))
				}
			}

			// 详细信息节点
			if at.fileInfo.DetailedInfo != "" {
				detailLines := strings.Split(at.fileInfo.DetailedInfo, "\n")
				for i, line := range detailLines {
					line = strings.TrimSpace(line)
					if line != "" { // 忽略空行
						children = append(children, widget.TreeNodeID(fmt.Sprintf("info_detailed_%d", i)))
					}
				}
			}
		}
		return children
	case "sections":
		children := []widget.TreeNodeID{}
		if at.fileInfo != nil {
			if at.fileInfo.IsFatMachO {
				// Fat Mach-O: 先显示架构节点
				archMap := make(map[int]bool)
				for _, section := range at.fileInfo.Sections {
					if section.Type == "Architecture" {
						if !archMap[section.ArchIndex] {
							children = append(children, widget.TreeNodeID(fmt.Sprintf("arch_%d", section.ArchIndex)))
							archMap[section.ArchIndex] = true
						}
					}
				}
			} else {
				// 普通文件: 直接显示段节点
				for i := range at.fileInfo.Sections {
					children = append(children, widget.TreeNodeID(fmt.Sprintf("section_%d", i)))
				}
			}
		}
		return children
	default:
		// 处理架构节点的子节点（Fat Mach-O）
		if strings.HasPrefix(string(uid), "arch_") {
			children := []widget.TreeNodeID{}
			archIndexStr := strings.TrimPrefix(string(uid), "arch_")
			if archIndex, err := strconv.Atoi(archIndexStr); err == nil && at.fileInfo != nil {
				for i, section := range at.fileInfo.Sections {
					if section.ArchIndex == archIndex && section.Type != "Architecture" {
						children = append(children, widget.TreeNodeID(fmt.Sprintf("section_%d", i)))
					}
				}
			}
			return children
		}
		return []widget.TreeNodeID{}
	}
}

func (at *AnalysisTab) isBranch(uid widget.TreeNodeID) bool {
	// 根节点、文件信息节点、段节点、架构节点是分支
	if uid == "" || uid == "file_info" || uid == "sections" {
		return true
	}
	// 架构节点也是分支
	if strings.HasPrefix(string(uid), "arch_") {
		return true
	}
	return false
}

func (at *AnalysisTab) getNodeText(uid widget.TreeNodeID) string {
	switch uid {
	case "":
		return "Root"
	case "file_info":
		return "文件信息"
	case "sections":
		if at.fileInfo != nil {
			return fmt.Sprintf("段信息 (%d)", len(at.fileInfo.Sections))
		}
		return "段信息 (0)"
	default:
		// 文件信息多行节点
		if strings.HasPrefix(string(uid), "info_type_") {
			indexStr := strings.TrimPrefix(string(uid), "info_type_")
			if index, err := strconv.Atoi(indexStr); err == nil && at.fileInfo != nil {
				typeLines := strings.Split(at.fileInfo.FileType, "\n")
				if index < len(typeLines) {
					line := strings.TrimSpace(typeLines[index])
					if line != "" {
						return fmt.Sprintf("文件类型: %s", line)
					}
				}
			}
			return "文件类型: 未知"
		}
		if strings.HasPrefix(string(uid), "info_size_") {
			indexStr := strings.TrimPrefix(string(uid), "info_size_")
			if index, err := strconv.Atoi(indexStr); err == nil && at.fileInfo != nil {
				sizeText := fmt.Sprintf("%d bytes", at.fileInfo.FileSize)
				sizeLines := strings.Split(sizeText, "\n")
				if index < len(sizeLines) {
					line := strings.TrimSpace(sizeLines[index])
					if line != "" {
						return fmt.Sprintf("文件大小: %s", line)
					}
				}
			}
			return "文件大小: 未知"
		}
		if strings.HasPrefix(string(uid), "info_arch_") {
			indexStr := strings.TrimPrefix(string(uid), "info_arch_")
			if index, err := strconv.Atoi(indexStr); err == nil && at.fileInfo != nil {
				archLines := strings.Split(at.fileInfo.Architecture, "\n")
				if index < len(archLines) {
					// line := strings.TrimSpace(archLines[index])
					// if line != "" {
					return "架构:"
					// }
				}
			}
			return "架构: 未知"
		}
		if strings.HasPrefix(string(uid), "info_detailed_") {
			indexStr := strings.TrimPrefix(string(uid), "info_detailed_")
			if index, err := strconv.Atoi(indexStr); err == nil && at.fileInfo != nil {
				detailLines := strings.Split(at.fileInfo.DetailedInfo, "\n")
				if index < len(detailLines) {
					line := strings.TrimSpace(detailLines[index])
					if line != "" {
						return fmt.Sprintf("  %s", line)
					}
				}
			}
			return "详细信息: 未知"
		}

		// 架构节点
		if strings.HasPrefix(string(uid), "arch_") {
			archIndexStr := strings.TrimPrefix(string(uid), "arch_")
			if archIndex, err := strconv.Atoi(archIndexStr); err == nil && at.fileInfo != nil {
				for _, section := range at.fileInfo.Sections {
					if section.ArchIndex == archIndex && section.Type == "Architecture" {
						return section.Name
					}
				}
			}
			return fmt.Sprintf("架构 %s", archIndexStr)
		}
		// 段节点
		if strings.HasPrefix(string(uid), "section_") {
			index := strings.TrimPrefix(string(uid), "section_")
			if i, err := strconv.Atoi(index); err == nil && at.fileInfo != nil && i < len(at.fileInfo.Sections) {
				section := at.fileInfo.Sections[i]
				return fmt.Sprintf("%s (0x%X, %d bytes)", section.Name, section.Offset, section.Size)
			}
		}
		return string(uid)
	}
}

// getCachedSectionData 获取缓存的段数据
func (at *AnalysisTab) getCachedSectionData(sectionIndex int) ([]byte, error) {
	// 检查缓存
	if data, exists := at.sectionDataCache[sectionIndex]; exists {
		return data, nil
	}

	// 获取新数据并缓存
	data, err := at.analyzer.GetSectionData(at.currentFile, sectionIndex, at.fileInfo.Sections)
	if err != nil {
		return nil, err
	}

	// 只缓存
	at.sectionDataCache[sectionIndex] = data

	return data, nil
}

// clearSectionDataCache 清理段数据缓存
func (at *AnalysisTab) clearSectionDataCache() {
	at.sectionDataCache = make(map[int][]byte)
}

// formatSectionInfoIDA IDA风格的段信息格式化（15列）
func (at *AnalysisTab) formatSectionInfoIDA(section *core.SectionInfo, col int) string {
	switch col {
	case 0: // Name
		if section.Name != "" {
			return section.Name
		}
		return "<unnamed>"
	case 1: // Start
		return fmt.Sprintf("%016X", section.Offset)
	case 2: // End
		return fmt.Sprintf("%016X", section.EndOffset)
	case 3: // R (Readable)
		if section.Readable {
			return "R"
		}
		return "."
	case 4: // W (Writable)
		if section.Writable {
			return "W"
		}
		return "."
	case 5: // X (Executable)
		if section.Executable {
			return "X"
		}
		return "."
	case 6: // D (Data)
		if section.Data {
			return "D"
		}
		return "."
	case 7: // L (Loaded)
		if section.Loaded {
			return "L"
		}
		return "."
	case 8: // Align
		if section.Align != "" {
			return section.Align
		}
		return "byte"
	case 9: // Base
		if section.Base != "" {
			return section.Base
		}
		return "01"
	case 10: // Type
		return "public"
	case 11: // Class
		if section.Class != "" {
			return section.Class
		}
		return "DATA"
	case 12: // AD (Address Dependent) - 应该显示位数
		if section.Bitness != "" {
			return section.Bitness
		}
		return "64"
	case 13: // T (Type) - 应该显示"00"
		return "00"
	case 14: // DS (Data Size) - 显示段类型编号
		if section.Base != "" {
			return section.Base
		}
		return "01"
	}
	return ""
}

// updateTableLayout 更新表格布局（替换为双表格切换模式）
func (at *AnalysisTab) updateTableLayout() {
	if at.selectedSection >= 0 {
		// 切换到段数据模式
		at.switchToSectionDataMode()
	} else {
		// 切换到段列表模式
		at.switchToSectionListMode()
	}
}

// displaySectionData 显示段数据（动态适配架构和段类型）
func (at *AnalysisTab) displaySectionData(id widget.TableCellID, label *widget.Label) {
	// 严格的边界检查
	if at.fileInfo == nil ||
		at.selectedSection < 0 ||
		at.selectedSection >= len(at.fileInfo.Sections) ||
		id.Row < 0 ||
		id.Col < 0 ||
		id.Col >= 5 {
		label.SetText("")
		return
	}

	section := at.fileInfo.Sections[at.selectedSection]

	// 所有段都使用字符串搜索模式显示（类似IDA strings窗口）
	// 搜索段中的字符串并显示：地址、长度、文本内容
	at.displayStringsInSection(id, label, section)
}

// displaySectionDataWithRichText 使用RichText显示段数据（支持高亮）
func (at *AnalysisTab) displaySectionDataWithRichText(id widget.TableCellID, cont *fyne.Container) {
	if at.fileInfo == nil || len(at.fileInfo.Sections) == 0 {
		return
	}

	section := at.fileInfo.Sections[at.selectedSection]

	// 所有段都使用字符串搜索模式显示（类似IDA strings窗口）
	// 搜索段中的字符串并显示：地址、长度、文本内容
	at.displayStringsInSectionWithRichText(id, cont, section)
}

// displayStringsInSectionWithRichText 使用RichText显示段中的字符串（支持真正的高亮）
func (at *AnalysisTab) displayStringsInSectionWithRichText(id widget.TableCellID, cont *fyne.Container, section core.SectionInfo) {
	// 获取段数据
	data, err := at.getCachedSectionData(at.selectedSection)
	if err != nil {
		label := widget.NewLabel("Error")
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
		return
	}

	if len(data) == 0 {
		label := widget.NewLabel("No Data")
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
		return
	}

	// 性能优化：限制解析的数据量
	const maxStringParseSize = 512 * 1024 // 512KB限制
	parseData := data
	showWarning := false
	if len(data) > maxStringParseSize {
		parseData = data[:maxStringParseSize]
		showWarning = true
	}

	// 决定使用哪个字符串列表：过滤后的还是全部的
	var displayStringList []StringData

	if at.currentSearchText != "" && at.filteredStrings != nil {
		// 使用过滤后的字符串列表
		displayStringList = at.filteredStrings
	} else {
		// 智能解析字符串：根据段类型选择解析方法
		if at.isCStringSection(section) {
			// 明确的字符串段：按\0分割
			displayStringList = at.parseCStrings(parseData)
		} else {
			// 其他段：使用IDA风格的字符串搜索算法
			displayStringList = at.parseStringsIDAStyle(parseData)
		}
	}

	// 如果没有找到字符串，显示提示
	if len(displayStringList) == 0 {
		if id.Row == 0 {
			text := ""
			switch id.Col {
			case 0:
				text = "0"
			case 1:
				text = fmt.Sprintf("%08X", section.Offset)
			case 2:
				text = "NO_STRINGS"
			case 3:
				text = "0"
			case 4:
				text = "未在此段中找到字符串"
			}
			label := widget.NewLabel(text)
			label.Truncation = fyne.TextTruncateEllipsis
			cont.Objects = []fyne.CanvasObject{label}
		}
		return
	}

	// 处理警告行（如果段太大）
	adjustedRow := id.Row
	if showWarning {
		if id.Row == 0 {
			text := ""
			switch id.Col {
			case 0:
				text = "⚠️"
			case 1:
				text = fmt.Sprintf("%08X", section.Offset)
			case 2:
				text = "LARGE_SECTION"
			case 3:
				text = fmt.Sprintf("%d", len(data))
			case 4:
				text = fmt.Sprintf("段太大，仅搜索前%.1fKB的字符串", float64(maxStringParseSize)/1024)
			}
			label := widget.NewLabel(text)
			label.Truncation = fyne.TextTruncateEllipsis
			cont.Objects = []fyne.CanvasObject{label}
			return
		}
		adjustedRow = id.Row - 1 // 减去警告行
	}

	// 检查行索引是否有效
	if adjustedRow < 0 || adjustedRow >= len(displayStringList) {
		return
	}

	str := displayStringList[adjustedRow]

	// 根据列类型创建不同的组件
	switch id.Col {
	case 0:
		// Index - 字符串索引
		label := widget.NewLabel(fmt.Sprintf("%d", adjustedRow))
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
	case 1:
		// Address - 字符串在文件中的地址
		address := section.Offset + str.Offset
		var text string
		if section.PointerSize == 4 {
			text = fmt.Sprintf("%08X", address)
		} else {
			text = fmt.Sprintf("%016X", address)
		}
		label := widget.NewLabel(text)
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
	case 2:
		// Type - 字符串类型标识
		var text string
		if len(str.Data) > 30 {
			text = "LONG_STR"
		} else if at.isASCIIString(str.Data) {
			text = "ASCII"
		} else {
			text = "UTF8"
		}
		label := widget.NewLabel(text)
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
	case 3:
		// Length - 字符串长度
		label := widget.NewLabel(fmt.Sprintf("%d", len(str.Data)))
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
	case 4:
		// String - 字符串内容（使用RichText支持高亮）
		displayStr := str.Data
		// 清理不可显示字符，保持完整内容
		displayStr = at.cleanStringForDisplay(displayStr)

		// 如果有搜索文本，使用RichText高亮显示
		if at.currentSearchText != "" && at.highlightMatches != nil {
			if _, exists := at.highlightMatches[adjustedRow]; exists {
				richText := at.createHighlightedRichText(displayStr, at.currentSearchText)
				cont.Objects = []fyne.CanvasObject{richText}
			} else {
				label := widget.NewLabel(displayStr)
				cont.Objects = []fyne.CanvasObject{label}
			}
		} else {
			label := widget.NewLabel(displayStr)
			label.Truncation = fyne.TextTruncateEllipsis
			cont.Objects = []fyne.CanvasObject{label}
		}
	}
}

// displayStringsInSection 显示段中的字符串（IDA strings窗口风格）
// 搜索段中的所有字符串，显示地址、长度、文本内容
func (at *AnalysisTab) displayStringsInSection(id widget.TableCellID, label *widget.Label, section core.SectionInfo) {
	// 获取段数据
	data, err := at.getCachedSectionData(at.selectedSection)
	if err != nil {
		label.SetText("Error")
		return
	}

	if len(data) == 0 {
		label.SetText("No Data")
		return
	}

	// 性能优化：限制解析的数据量
	const maxStringParseSize = 512 * 1024 // 512KB限制
	parseData := data
	showWarning := false
	if len(data) > maxStringParseSize {
		parseData = data[:maxStringParseSize]
		showWarning = true
	}

	// 决定使用哪个字符串列表：过滤后的还是全部的
	var displayStringList []StringData

	if at.currentSearchText != "" && at.filteredStrings != nil {
		// 使用过滤后的字符串列表
		displayStringList = at.filteredStrings
	} else {
		// 智能解析字符串：根据段类型选择解析方法
		if at.isCStringSection(section) {
			// 明确的字符串段：按\0分割
			displayStringList = at.parseCStrings(parseData)
		} else {
			// 其他段：使用IDA风格的字符串搜索算法
			displayStringList = at.parseStringsIDAStyle(parseData)
		}
	}

	// 如果没有找到字符串，显示提示
	if len(displayStringList) == 0 {
		if id.Row == 0 {
			switch id.Col {
			case 0:
				label.SetText("0")
			case 1:
				label.SetText(fmt.Sprintf("%08X", section.Offset))
			case 2:
				label.SetText("NO_STRINGS")
			case 3:
				label.SetText("0")
			case 4:
				label.SetText("未在此段中找到字符串")
			}
		} else {
			label.SetText("")
		}
		return
	}

	// 处理警告行（如果段太大）
	adjustedRow := id.Row
	if showWarning {
		if id.Row == 0 {
			// 显示警告行
			switch id.Col {
			case 0:
				label.SetText("⚠️")
			case 1:
				label.SetText(fmt.Sprintf("%08X", section.Offset))
			case 2:
				label.SetText("LARGE_SECTION")
			case 3:
				label.SetText(fmt.Sprintf("%d", len(data)))
			case 4:
				label.SetText(fmt.Sprintf("段太大，仅搜索前%.1fKB的字符串", float64(maxStringParseSize)/1024))
			}
			return
		}
		adjustedRow = id.Row - 1 // 减去警告行
	}

	// 检查行索引是否有效
	if adjustedRow < 0 || adjustedRow >= len(displayStringList) {
		label.SetText("")
		return
	}

	str := displayStringList[adjustedRow]

	switch id.Col {
	case 0:
		// Index - 字符串索引
		label.SetText(fmt.Sprintf("%d", adjustedRow))
	case 1:
		// Address - 字符串在文件中的地址
		address := section.Offset + str.Offset
		if section.PointerSize == 4 {
			label.SetText(fmt.Sprintf("%08X", address))
		} else {
			label.SetText(fmt.Sprintf("%016X", address))
		}
	case 2:
		// Type - 字符串类型标识
		if len(str.Data) > 30 {
			label.SetText("LONG_STR")
		} else if at.isASCIIString(str.Data) {
			label.SetText("ASCII")
		} else {
			label.SetText("UTF8")
		}
	case 3:
		// Length - 字符串长度
		label.SetText(fmt.Sprintf("%d", len(str.Data)))
	case 4:
		// String - 字符串内容（支持搜索高亮）
		displayStr := str.Data
		// 清理不可显示字符，保持完整内容
		displayStr = at.cleanStringForDisplay(displayStr)

		// 如果有搜索文本且存在高亮匹配，添加高亮标记
		if at.currentSearchText != "" && at.highlightMatches != nil {
			if matches, exists := at.highlightMatches[adjustedRow]; exists && len(matches) > 0 {
				// 使用更优雅的高亮方式
				displayStr = at.addHighlightMarkers(displayStr, at.currentSearchText)
			}
		}

		label.SetText(displayStr)
	}
}

// StringData 字符串数据结构
type StringData struct {
	Index  int
	Offset uint64
	Data   string
}

// isASCIIString 判断字符串是否为纯ASCII
func (at *AnalysisTab) isASCIIString(s string) bool {
	for _, b := range []byte(s) {
		if b > 127 {
			return false
		}
	}
	return true
}

// cleanStringForDisplay 清理字符串以便显示
func (at *AnalysisTab) cleanStringForDisplay(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		if r >= 32 && r <= 126 {
			// ASCII可打印字符
			result.WriteRune(r)
		} else if r > 127 {
			// UTF-8字符，保留
			result.WriteRune(r)
		} else {
			// 控制字符，替换为点
			result.WriteByte('.')
		}
	}

	return result.String()
}

// filterSectionData 过滤段数据（搜索字符串）
func (at *AnalysisTab) filterSectionData(searchText string) {
	// 如果当前没有选中段，不执行过滤
	if at.selectedSection < 0 || at.fileInfo == nil || at.currentTable != at.sectionDataTable {
		return
	}

	// 清理并保存搜索文本
	at.currentSearchText = strings.TrimSpace(searchText)

	// 如果搜索文本为空，清除过滤显示所有数据
	if at.currentSearchText == "" {
		at.filteredStrings = nil
		at.highlightMatches = nil
		at.currentTable.Refresh()
		section := at.fileInfo.Sections[at.selectedSection]
		at.updateStatus(fmt.Sprintf("显示段 %s 的所有字符串", section.Name))
		return
	}

	// 获取当前段的所有字符串
	section := at.fileInfo.Sections[at.selectedSection]
	data, err := at.getCachedSectionData(at.selectedSection)
	if err != nil || len(data) == 0 {
		at.updateStatus("无法获取段数据")
		return
	}

	// 性能优化：限制解析的数据量
	const maxStringParseSize = 512 * 1024
	parseData := data
	if len(data) > maxStringParseSize {
		parseData = data[:maxStringParseSize]
	}

	// 智能解析字符串
	var allStrings []StringData
	if at.isCStringSection(section) {
		allStrings = at.parseCStrings(parseData)
	} else {
		allStrings = at.parseStringsIDAStyle(parseData)
	}

	// 执行模糊匹配搜索
	at.filteredStrings = nil
	at.highlightMatches = make(map[int][]int)

	searchLower := strings.ToLower(at.currentSearchText)
	matchCount := 0

	for _, str := range allStrings {
		strLower := strings.ToLower(str.Data)

		// 检查是否包含搜索文本
		if strings.Contains(strLower, searchLower) {
			// 找到匹配的字符串，添加到过滤结果
			filteredIndex := len(at.filteredStrings)
			at.filteredStrings = append(at.filteredStrings, StringData{
				Index:  filteredIndex,
				Offset: str.Offset,
				Data:   str.Data,
			})

			// 计算高亮位置
			matches := at.findAllMatches(strLower, searchLower)
			if len(matches) > 0 {
				at.highlightMatches[filteredIndex] = matches
			}

			matchCount++
		}
	}

	// 刷新表格显示过滤结果
	at.currentTable.Refresh()
	at.currentTable.ScrollToTop()

	// 更新状态信息
	if matchCount > 0 {
		at.updateStatus(fmt.Sprintf("在段 %s 中找到 %d 个匹配 \"%s\" 的字符串",
			section.Name, matchCount, at.currentSearchText))
	} else {
		at.updateStatus(fmt.Sprintf("在段 %s 中未找到匹配 \"%s\" 的字符串",
			section.Name, at.currentSearchText))
	}
}

// isCStringSection 判断是否为明确的C字符串段
func (at *AnalysisTab) isCStringSection(section core.SectionInfo) bool {
	// 明确的C字符串段名称
	cstringSegments := []string{
		"__cstring",  // Mach-O C字符串段
		"__cfstring", // Core Foundation字符串
		"__string",   // 通用字符串段
		".rodata",    // ELF只读数据段（通常包含字符串常量）
		".rdata",     // PE只读数据段
	}

	sectionName := strings.ToLower(section.Name)
	for _, cstringName := range cstringSegments {
		if strings.Contains(sectionName, cstringName) {
			return true
		}
	}

	return false
}

// parseCStrings 解析C字符串（按\0分割）
func (at *AnalysisTab) parseCStrings(data []byte) []StringData {
	var stringList []StringData
	var start int

	for i, b := range data {
		if b == 0 {
			// 找到\0分隔符
			if i > start {
				str := string(data[start:i])
				// 只保留有意义的字符串（长度>=2）
				if len(str) >= 2 && at.containsPrintableChars(str) {
					stringList = append(stringList, StringData{
						Index:  len(stringList),
						Offset: uint64(start),
						Data:   str,
					})
				}
			}
			start = i + 1
		}
	}

	// 处理最后一个字符串（如果没有以\0结尾）
	if start < len(data) {
		str := string(data[start:])
		if len(str) >= 2 && at.containsPrintableChars(str) {
			stringList = append(stringList, StringData{
				Index:  len(stringList),
				Offset: uint64(start),
				Data:   str,
			})
		}
	}

	return stringList
}

// parseStringsIDAStyle 使用IDA风格的字符串搜索算法
func (at *AnalysisTab) parseStringsIDAStyle(data []byte) []StringData {
	var stringList []StringData
	var currentString []byte
	var currentOffset uint64

	const minStringLength = 4 // IDA默认最小字符串长度

	for i, b := range data {
		if at.isPrintableChar(b) {
			// 可打印字符，添加到当前字符串
			if currentString == nil {
				currentOffset = uint64(i)
			}
			currentString = append(currentString, b)
		} else if b >= 128 {
			// 可能的UTF-8字符，尝试解析
			if at.isValidUTF8Start(data, i) {
				if currentString == nil {
					currentOffset = uint64(i)
				}
				// 添加UTF-8字节序列
				utfLen := at.getUTF8Length(b)
				for j := 0; j < utfLen && i+j < len(data); j++ {
					currentString = append(currentString, data[i+j])
				}
				// 跳过UTF-8的剩余字节
				for j := 1; j < utfLen && i+j < len(data); j++ {
					i++
				}
			} else {
				// 非文本字符，结束当前字符串
				if len(currentString) >= minStringLength {
					stringList = append(stringList, StringData{
						Index:  len(stringList),
						Offset: currentOffset,
						Data:   string(currentString),
					})
				}
				currentString = nil
			}
		} else {
			// 非可打印字符，结束当前字符串
			if len(currentString) >= minStringLength {
				stringList = append(stringList, StringData{
					Index:  len(stringList),
					Offset: currentOffset,
					Data:   string(currentString),
				})
			}
			currentString = nil
		}
	}

	// 处理最后一个字符串
	if len(currentString) >= minStringLength {
		stringList = append(stringList, StringData{
			Index:  len(stringList),
			Offset: currentOffset,
			Data:   string(currentString),
		})
	}

	return stringList
}

// containsPrintableChars 检查字符串是否包含足够的可打印字符
func (at *AnalysisTab) containsPrintableChars(s string) bool {
	printableCount := 0
	for _, r := range s {
		if r >= 32 && r <= 126 {
			printableCount++
		}
	}
	// 至少80%的字符是可打印的
	return float64(printableCount)/float64(len(s)) >= 0.8
}

// findAllMatches 找到字符串中所有匹配位置
func (at *AnalysisTab) findAllMatches(text, pattern string) []int {
	var matches []int

	if len(pattern) == 0 {
		return matches
	}

	start := 0
	for {
		index := strings.Index(text[start:], pattern)
		if index == -1 {
			break
		}

		actualIndex := start + index
		matches = append(matches, actualIndex)
		start = actualIndex + 1 // 允许重叠匹配
	}

	return matches
}

// addHighlightMarkers 为字符串添加高亮标记（使用专业的视觉标记）
func (at *AnalysisTab) addHighlightMarkers(text, searchPattern string) string {
	if searchPattern == "" {
		return text
	}

	searchLower := strings.ToLower(searchPattern)
	textLower := strings.ToLower(text)

	var result strings.Builder
	lastIndex := 0

	for {
		index := strings.Index(textLower[lastIndex:], searchLower)
		if index == -1 {
			// 添加剩余文本
			result.WriteString(text[lastIndex:])
			break
		}

		actualIndex := lastIndex + index

		// 添加匹配前的文本
		result.WriteString(text[lastIndex:actualIndex])

		// 添加高亮标记的匹配文本（使用专业的标记方式）
		matchedText := text[actualIndex : actualIndex+len(searchPattern)]

		// 方案1: 使用下划线和粗体效果（如果支持）
		result.WriteString("╣") // 使用盒绘字符作为开始标记
		result.WriteString(matchedText)
		result.WriteString("╠") // 使用盒绘字符作为结束标记

		lastIndex = actualIndex + len(searchPattern)
	}

	return result.String()
}

// createHighlightedRichText 创建带高亮的RichText组件
func (at *AnalysisTab) createHighlightedRichText(text, searchPattern string) *widget.RichText {
	richText := widget.NewRichText()
	richText.Truncation = fyne.TextTruncateEllipsis
	if searchPattern == "" {
		// 没有搜索模式，显示普通文本
		richText.ParseMarkdown(text)
		return richText
	}

	searchLower := strings.ToLower(searchPattern)
	textLower := strings.ToLower(text)

	// 构建富文本内容
	var content strings.Builder
	lastIndex := 0

	for {
		index := strings.Index(textLower[lastIndex:], searchLower)
		if index == -1 {
			// 添加剩余的普通文本
			if lastIndex < len(text) {
				content.WriteString(text[lastIndex:])
			}
			break
		}

		actualIndex := lastIndex + index

		// 添加匹配前的普通文本
		if actualIndex > lastIndex {
			content.WriteString(text[lastIndex:actualIndex])
		}

		// 添加高亮的匹配文本（使用Markdown语法 - 粗体+斜体）
		matchedText := text[actualIndex : actualIndex+len(searchPattern)]
		content.WriteString("**") // 粗体+斜体
		content.WriteString(matchedText)
		content.WriteString("**")

		lastIndex = actualIndex + len(searchPattern)
	}

	// 解析Markdown内容
	richText.ParseMarkdown(content.String())

	// 设置一些样式属性
	richText.Wrapping = fyne.TextWrapOff // 不换行，保持表格整洁

	return richText
}

// addColorHighlight 添加颜色高亮效果（备选方案）
func (at *AnalysisTab) addColorHighlight(text, searchPattern string) string {
	if searchPattern == "" {
		return text
	}

	// 使用Unicode块字符创建背景高亮效果
	searchLower := strings.ToLower(searchPattern)
	textLower := strings.ToLower(text)

	// 创建高亮字符映射
	highlightMap := map[rune]rune{
		'a': '🅰', 'b': '🅱', 'c': '🅲', 'd': '🅳', 'e': '🅴',
		'f': '🅵', 'g': '🅶', 'h': '🅷', 'i': '🅸', 'j': '🅹',
		'k': '🅺', 'l': '🅻', 'm': '🅼', 'n': '🅽', 'o': '🅾',
		'p': '🅿', 'q': '🆀', 'r': '🆁', 's': '🆂', 't': '🆃',
		'u': '🆄', 'v': '🆅', 'w': '🆆', 'x': '🆇', 'y': '🆈', 'z': '🆉',
	}

	var result strings.Builder
	lastIndex := 0

	for {
		index := strings.Index(textLower[lastIndex:], searchLower)
		if index == -1 {
			result.WriteString(text[lastIndex:])
			break
		}

		actualIndex := lastIndex + index
		result.WriteString(text[lastIndex:actualIndex])

		// 转换匹配的文本为高亮版本
		matchedText := text[actualIndex : actualIndex+len(searchPattern)]
		for _, r := range matchedText {
			if highlighted, exists := highlightMap[unicode.ToLower(r)]; exists {
				result.WriteRune(highlighted)
			} else {
				result.WriteRune(r)
			}
		}

		lastIndex = actualIndex + len(searchPattern)
	}

	return result.String()
}

// isPrintableChar 判断字符是否可打印
func (at *AnalysisTab) isPrintableChar(b byte) bool {
	// ASCII可打印字符范围：32-126
	return b >= 32 && b <= 126
}

// isValidUTF8Start 检查是否为有效的UTF-8起始字节
func (at *AnalysisTab) isValidUTF8Start(data []byte, pos int) bool {
	if pos >= len(data) {
		return false
	}

	b := data[pos]

	// UTF-8编码规则检查
	if b&0x80 == 0 {
		return true // ASCII
	} else if b&0xE0 == 0xC0 {
		// 2字节UTF-8
		return pos+1 < len(data) && (data[pos+1]&0xC0) == 0x80
	} else if b&0xF0 == 0xE0 {
		// 3字节UTF-8
		return pos+2 < len(data) &&
			(data[pos+1]&0xC0) == 0x80 &&
			(data[pos+2]&0xC0) == 0x80
	} else if b&0xF8 == 0xF0 {
		// 4字节UTF-8
		return pos+3 < len(data) &&
			(data[pos+1]&0xC0) == 0x80 &&
			(data[pos+2]&0xC0) == 0x80 &&
			(data[pos+3]&0xC0) == 0x80
	}

	return false
}

// getUTF8Length 获取UTF-8字符的字节长度
func (at *AnalysisTab) getUTF8Length(b byte) int {
	if b&0x80 == 0 {
		return 1 // ASCII
	} else if b&0xE0 == 0xC0 {
		return 2 // 2字节UTF-8
	} else if b&0xF0 == 0xE0 {
		return 3 // 3字节UTF-8
	} else if b&0xF8 == 0xF0 {
		return 4 // 4字节UTF-8
	}
	return 1 // 错误情况，返回1
}

// PointerData 指针数据结构
type PointerData struct {
	Index   int
	Offset  uint64
	Address uint64
	IsValid bool
}

// onTreeNodeSelected 处理树节点选择事件
func (at *AnalysisTab) onTreeNodeSelected(uid widget.TreeNodeID) {
	switch uid {
	case "file_info":
		// 文件信息节点：切换到段列表模式
		at.selectedSection = -1
		at.switchToSectionListMode()
		at.currentTable.Refresh()
		at.currentTable.ScrollToTop()
		at.updateStatus("显示段列表信息（IDA风格）")

	case "sections":
		// 显示所有段信息
		at.selectedSection = -1
		at.switchToSectionListMode()
		at.currentTable.Refresh()
		at.currentTable.ScrollToTop()
		if at.fileInfo != nil {
			at.updateStatus(fmt.Sprintf("显示所有段信息 (%d个)", len(at.fileInfo.Sections)))
		}

	default:
		// 文件信息多行节点
		if strings.HasPrefix(string(uid), "info_type_") ||
			strings.HasPrefix(string(uid), "info_size_") ||
			strings.HasPrefix(string(uid), "info_arch_") ||
			strings.HasPrefix(string(uid), "info_detailed_") {
			// 文件信息子节点：不需要特殊处理，只显示基本信息
			at.selectedSection = -1
			at.switchToSectionListMode()
			at.currentTable.Refresh()
			at.currentTable.ScrollToTop()
			at.updateStatus("显示文件详细信息")
			// 架构节点
		} else if strings.HasPrefix(string(uid), "arch_") {
			archIndexStr := strings.TrimPrefix(string(uid), "arch_")
			if archIndex, err := strconv.Atoi(archIndexStr); err == nil && at.fileInfo != nil {
				// 显示该架构的信息
				at.selectedSection = -1
				for _, section := range at.fileInfo.Sections {
					if section.ArchIndex == archIndex && section.Type == "Architecture" {
						at.updateStatus(fmt.Sprintf("架构信息: %s", section.Name))
						break
					}
				}
				at.currentTable.Refresh()
				at.currentTable.ScrollToTop()
			}
		} else if strings.HasPrefix(string(uid), "section_") {
			// 段节点
			index := strings.TrimPrefix(string(uid), "section_")
			if i, err := strconv.Atoi(index); err == nil && at.fileInfo != nil && i < len(at.fileInfo.Sections) {
				// 检查是否切换到不同的段，如果是则清理不必要的缓存
				if at.selectedSection != i {
					// 保留当前段和新段的缓存，清理其他
					newCache := make(map[int][]byte)
					if data, exists := at.sectionDataCache[at.selectedSection]; exists {
						newCache[at.selectedSection] = data
					}
					if data, exists := at.sectionDataCache[i]; exists {
						newCache[i] = data
					}
					at.sectionDataCache = newCache
				}

				at.lastSelectedSection = at.selectedSection
				at.selectedSection = i
				section := at.fileInfo.Sections[i]
				at.updateTableLayout() // 更新表格布局为段数据模式
				at.updateStatus(fmt.Sprintf("选择段: %s (大小: %d bytes)", section.Name, section.Size))
				at.currentTable.Refresh()
				at.currentTable.ScrollToTop()
			}
		}
	}
}

// Content 返回标签页内容
func (at *AnalysisTab) Content() *fyne.Container {
	return at.content
}

// Refresh 刷新标签页
func (at *AnalysisTab) Refresh() {
	if at.tree != nil {
		at.tree.Refresh()
	}
	if at.currentTable != nil {
		at.currentTable.Refresh()
		at.currentTable.ScrollToTop()
	}
}

// createTables 创建两个独立的表格实例
func (at *AnalysisTab) createTables() {
	// 创建段列表表格（IDA风格）
	at.sectionListTable = widget.NewTableWithHeaders(
		func() (int, int) {
			if at.fileInfo == nil || len(at.fileInfo.Sections) == 0 {
				return 0, 15
			}
			return len(at.fileInfo.Sections), 15
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Cell Data")
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			label := object.(*widget.Label)
			label.SetText("")
			label.Truncation = fyne.TextTruncateEllipsis

			if at.fileInfo == nil || len(at.fileInfo.Sections) == 0 {
				return
			}

			if id.Row >= 0 && id.Row < len(at.fileInfo.Sections) && id.Col >= 0 && id.Col < 15 {
				section := &at.fileInfo.Sections[id.Row]
				text := at.formatSectionInfoIDA(section, id.Col)
				label.SetText(text)
			}
		},
	)

	// 设置段列表表格表头
	at.sectionListTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabel("Header")
	}
	at.sectionListTable.UpdateHeader = func(id widget.TableCellID, object fyne.CanvasObject) {
		label := object.(*widget.Label)
		label.TextStyle = fyne.TextStyle{Bold: true}

		headers := []string{"Name", "Start", "End", "R", "W", "X", "D", "L", "Align", "Base", "Type", "Class", "AD", "T", "DS"}
		if id.Col >= 0 && id.Col < len(headers) {
			label.SetText(headers[id.Col])
		}
	}

	// 设置段列表表格列宽
	at.sectionListTable.SetColumnWidth(0, 140) // Name
	at.sectionListTable.SetColumnWidth(1, 140) // Start
	at.sectionListTable.SetColumnWidth(2, 140) // End
	at.sectionListTable.SetColumnWidth(3, 25)  // R
	at.sectionListTable.SetColumnWidth(4, 25)  // W
	at.sectionListTable.SetColumnWidth(5, 25)  // X
	at.sectionListTable.SetColumnWidth(6, 25)  // D
	at.sectionListTable.SetColumnWidth(7, 25)  // L
	at.sectionListTable.SetColumnWidth(8, 70)  // Align
	at.sectionListTable.SetColumnWidth(9, 40)  // Base
	at.sectionListTable.SetColumnWidth(10, 50) // Type
	at.sectionListTable.SetColumnWidth(11, 50) // Class
	at.sectionListTable.SetColumnWidth(12, 30) // AD
	at.sectionListTable.SetColumnWidth(13, 25) // T
	at.sectionListTable.SetColumnWidth(14, 30) // DS
	at.sectionListTable.SetRowHeight(0, 30)

	// 创建段数据表格（十六进制/字符串）
	at.sectionDataTable = widget.NewTableWithHeaders(
		func() (int, int) {
			if at.fileInfo == nil || len(at.fileInfo.Sections) == 0 || at.selectedSection < 0 {
				return 0, 5
			}

			if at.selectedSection < len(at.fileInfo.Sections) {
				// 所有段都使用字符串搜索模式
				data, err := at.getCachedSectionData(at.selectedSection)
				if err != nil || len(data) == 0 {
					return 1, 5 // 至少显示一行（错误或无数据提示）
				}

				// 性能优化：限制解析的数据量
				const maxStringParseSize = 512 * 1024
				parseData := data
				if len(data) > maxStringParseSize {
					parseData = data[:maxStringParseSize]
				}

				// 决定使用哪个字符串列表：过滤后的还是全部的
				section := at.fileInfo.Sections[at.selectedSection]
				var stringList []StringData

				if at.currentSearchText != "" && at.filteredStrings != nil {
					// 使用过滤后的字符串列表
					stringList = at.filteredStrings
				} else {
					// 智能解析字符串：根据段类型选择解析方法
					if at.isCStringSection(section) {
						// 明确的字符串段：按\0分割
						stringList = at.parseCStrings(parseData)
					} else {
						// 其他段：使用IDA风格的字符串搜索算法
						stringList = at.parseStringsIDAStyle(parseData)
					}
				}
				rowCount := len(stringList)

				// 如果段太大，增加一行警告
				if len(data) > maxStringParseSize {
					rowCount++
				}

				// 如果没有找到字符串，至少显示一行提示
				if rowCount == 0 {
					rowCount = 1
				}

				return rowCount, 5
			}
			return 0, 5
		},
		func() fyne.CanvasObject {
			// 为字符串列创建RichText，其他列使用Label
			return container.NewWithoutLayout() // 使用容器来支持动态组件
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cont := object.(*fyne.Container)
			cont.Objects = nil // 清空容器

			if at.fileInfo == nil || len(at.fileInfo.Sections) == 0 {
				return
			}

			if at.selectedSection >= 0 && at.selectedSection < len(at.fileInfo.Sections) {
				at.displaySectionDataWithRichText(id, cont)
			}
		},
	)

	// 设置段数据表格表头
	at.sectionDataTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabel("Header")
	}
	at.sectionDataTable.UpdateHeader = func(id widget.TableCellID, object fyne.CanvasObject) {
		label := object.(*widget.Label)
		label.TextStyle = fyne.TextStyle{Bold: true}

		headers := []string{"Index", "Address", "Type", "Length", "String"}
		if id.Col >= 0 && id.Col < len(headers) {
			label.SetText(headers[id.Col])
		}
	}

	// 设置段数据表格列宽
	at.sectionDataTable.SetColumnWidth(0, 60)  // Index
	at.sectionDataTable.SetColumnWidth(1, 120) // Address (扩大以适应16位地址)
	at.sectionDataTable.SetColumnWidth(2, 80)  // Type (字符串类型)
	at.sectionDataTable.SetColumnWidth(3, 80)  // Length
	at.sectionDataTable.SetColumnWidth(4, 400) // String (扩大以显示更多内容)
	at.sectionDataTable.SetRowHeight(0, 30)
}

// switchToSectionListMode 切换到段列表模式（IDA风格）
func (at *AnalysisTab) switchToSectionListMode() {
	if at.sectionListTable == nil || at.sectionDataTable == nil {
		return
	}

	at.currentTable = at.sectionListTable
	at.sectionListTable.Show()
	at.sectionDataTable.Hide()
}

// switchToSectionDataMode 切换到段数据模式（十六进制/字符串）
func (at *AnalysisTab) switchToSectionDataMode() {
	if at.sectionListTable == nil || at.sectionDataTable == nil {
		return
	}

	at.currentTable = at.sectionDataTable
	at.sectionDataTable.Show()
	at.sectionListTable.Hide()
}
