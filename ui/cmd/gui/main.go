package main

import (
	"fridare-gui/internal/assets"
	"fridare-gui/internal/config"
	"fridare-gui/internal/ui"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

const (
	AppID   = "com.suifei.fridare"
	AppName = "Fridare GUI"
	Version = "4.0.2"
)

func main() {
	// 设置应用元数据
	app.SetMetadata(fyne.AppMetadata{
		ID:      AppID,
		Name:    AppName,
		Version: Version,
	})

	// 创建应用程序
	myApp := app.NewWithID(AppID)

	// 设置应用程序图标
	myApp.SetIcon(assets.AppIcon)

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
