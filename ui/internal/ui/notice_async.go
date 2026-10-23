package ui

import (
	"strings"

	"fridare-gui/internal/i18n"
	"fridare-gui/internal/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NoticeData 通知数据结构
type NoticeData struct {
	Content string
	Error   error
}

// showNoticeAsync 异步显示通知对话框，使用协程和channel
func (mw *MainWindow) showNoticeAsync() {
	// 创建channel用于接收HTTP请求结果
	noticeChannel := make(chan NoticeData, 1)

	// 启动协程执行HTTP请求
	go func() {
		// 创建简单的对话框内容, 支持多行文本和markdown
		// 通知内容从 https://raw.githubusercontent.com/suifei/fridare/main/NOTICE.md 获取
		// 网络请求失败则不显示(自动挂接代理)

		// 从配置获取代理，如果配置没有，则尝试获取系统代理  ，否则为""
		// 系统代理获取：
		// HTTPProxy:  getEnvAny("HTTP_PROXY", "http_proxy"),
		// HTTPSProxy: getEnvAny("HTTPS_PROXY", "https_proxy"),
		// NoProxy:    getEnvAny("NO_PROXY", "no_proxy"),
		// CGI:        os.Getenv("REQUEST_METHOD") != "",
		noticeURL := "https://raw.githubusercontent.com/suifei/fridare/main/NOTICE.md"
		noticeContent, err := utils.FetchRemoteText(
			noticeURL,
			mw.config.Proxy)

		// 通过channel发送结果
		noticeChannel <- NoticeData{
			Content: noticeContent,
			Error:   err,
		}
	}()

	// 启动另一个协程监听channel并处理UI更新
	go func() {
		data := <-noticeChannel

		if data.Error != nil || strings.TrimSpace(data.Content) == "" {
			// 获取失败或内容为空则不显示通知
			return
		}

		// 在主UI线程中显示通知
		mw.addLog(i18n.T("INFO: 成功获取通知内容: https://raw.githubusercontent.com/suifei/fridare/main/NOTICE.md"))

		if mw.config.NoShowNotice {
			// 将通知显示到log中不弹窗，markdown 文本用于日志显示
			mw.addLog("NOTICE: " + strings.ReplaceAll(data.Content, "\n\n", "\n"))
			mw.addLog(i18n.T("INFO: 配置设置为不显示通知，跳过显示"))
			return
		}

		// 创建对话框
		contentViewer := widget.NewRichText()
		contentViewer.ParseMarkdown(data.Content)
		contentViewer.Wrapping = fyne.TextWrapWord

		// 支持对话框勾选不再显示并记录到配置文件
		checkbox := widget.NewCheck(i18n.T("不再显示此通知"), func(checked bool) {
			mw.config.NoShowNotice = checked
			// 保存配置
			if err := mw.config.Save(); err != nil {
				mw.updateStatus(i18n.T("保存配置失败: ") + err.Error())
				mw.addLog(i18n.T("ERROR: 保存配置失败: ") + err.Error())
			} else {
				mw.updateStatus(i18n.T("配置已保存"))
				mw.addLog(i18n.T("INFO: 配置已保存"))
			}
		})
		checkbox.SetChecked(mw.config.NoShowNotice)

		content := container.NewBorder(nil, checkbox, nil, nil, container.NewVScroll(contentViewer))

		dialog := dialog.NewCustom(i18n.T("Fridare GUI - 通知"), i18n.T("确定"), content, mw.window)
		dialog.Resize(fyne.NewSize(400, 400))
		dialog.Show()
	}()
}
