package main

import (
	"fridare-gui/internal/appmeta"
	"fridare-gui/internal/assets"
	"fridare-gui/internal/config"
	"fridare-gui/internal/ui"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// Windows PE icon + FileVersion/ProductName come from resource_windows_*.syso
// (generated via ui/scripts/gen-winres.ps1 from versioninfo.json + logo/AppIcon.ico).

func main() {
	// 设置应用元数据（Fyne 运行时 + macOS/Linux 打包）
	app.SetMetadata(fyne.AppMetadata{
		ID:      appmeta.GUIAppID,
		Name:    appmeta.GUIAppName,
		Version: appmeta.Version,
	})

	// 创建应用程序
	myApp := app.NewWithID(appmeta.GUIAppID)

	// 窗口/标题栏用 PNG；PE 文件图标由 .syso 提供
	myApp.SetIcon(assets.AppIconPNG)

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("加载配置失败，使用默认配置: %v", err)
		cfg = config.DefaultConfig()
	}

	// 创建主窗口
	mainWindow := ui.NewMainWindow(myApp, cfg)

	// 显示窗口并运行
	mainWindow.ShowAndRun()
}
