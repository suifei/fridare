package ui

import (
	"context"
	"fmt"
	"fridare-gui/internal/config"
	"fridare-gui/internal/core"
	"fridare-gui/internal/rebuild"
	"fridare-gui/internal/utils"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// RebuildTab is the optional Docker + AI source-rebuild surface.
// Layout: modern multi-column (nav | workspace | agent) instead of a single full-screen form.
// Pipeline is split into two steps:
//  1. Base image (toolchain only, local archive for reuse)
//  2. Develop (AI container: clone → mod → compile for a Frida version)
type RebuildTab struct {
	app          fyne.App
	config       *config.Config
	updateStatus StatusUpdater
	addLog       func(string)
	content      *fyne.Container
	window       fyne.Window

	// Step rail
	stepIndex     int // 0 = base image, 1 = develop
	step1Btn      *widget.Button
	step2Btn      *widget.Button
	stepBadge     *widget.Label
	imageStatus   *widget.Label
	mirrorEntry   *widget.Entry
	pullDirectChk *widget.Check
	archiveChk    *widget.Check
	enableSource  *widget.Check

	// Step 1
	bootstrapBtn *widget.Button
	refreshImgBtn *widget.Button

	// Step 2 form
	versionEntry     *widget.Entry
	magicEntry       *widget.Entry
	portEntry        *widget.Entry
	targetChecks     map[string]*widget.Check
	useGrokCheck     *widget.Check
	useGUIProxyCheck *widget.Check
	// profileSelect: safe | deep — deep = server+client protocol/API/idents sync
	profileSelect     *widget.Select
	randomAgentChk    *widget.Check
	developBtn        *widget.Button
	fullOneShotBtn    *widget.Button

	// Agent panel (always visible — not buried under scroll)
	goalsEntry    *widget.Entry
	agentLog      *LogView
	jobLog        *LogView
	progressBar   *widget.ProgressBar
	progressLabel *widget.Label
	artifactLabel *widget.Label
	depsLabel     *widget.Label

	// Actions
	cancelBtn    *widget.Button
	resetBtn     *widget.Button
	openArtsBtn  *widget.Button
	checkDepsBtn *widget.Button

	// Catalog browser (version/platform/magic — no rebuild needed)
	catalogList    *widget.List
	catalogStatus  *widget.Label
	catalogEntries []rebuild.CatalogEntry
	catalogSel     int
	refreshCatBtn  *widget.Button
	openCatRootBtn *widget.Button
	openCatSelBtn  *widget.Button
	installHint    *widget.Label

	// Panels for step switching
	step1Panel *fyne.Container
	step2Panel *fyne.Container
	workStack  *fyne.Container

	orch     *rebuild.Orchestrator
	lastDeps rebuild.DepsReport
	mu       sync.Mutex
	running  bool
}

// NewRebuildTab creates the source rebuild tab.
func NewRebuildTab(app fyne.App, cfg *config.Config, statusUpdater StatusUpdater, logFunc func(string), window fyne.Window) *RebuildTab {
	rt := &RebuildTab{
		app:          app,
		config:       cfg,
		updateStatus: statusUpdater,
		addLog:       logFunc,
		window:       window,
		targetChecks: make(map[string]*widget.Check),
		stepIndex:    0,
	}
	promptDir := filepath.Join(cfg.WorkDir, "rebuild", "agent-prompts")
	rt.orch = rebuild.NewOrchestrator(nil, rebuild.NewDefaultAgent(promptDir))
	rt.setupUI()
	return rt
}

func (rt *RebuildTab) setupUI() {
	// ── Top step rail ──────────────────────────────────────────────
	rt.step1Btn = widget.NewButton("① 基础镜像", func() { rt.selectStep(0) })
	rt.step2Btn = widget.NewButton("② 魔改开发", func() { rt.selectStep(1) })
	rt.stepBadge = widget.NewLabel("当前：步骤① 初始化工具链镜像")
	rt.stepBadge.TextStyle = fyne.TextStyle{Bold: true}
	rt.imageStatus = widget.NewLabel("镜像状态: 点击刷新…")
	rt.imageStatus.Wrapping = fyne.TextWrapWord

	// ── Shared enable + mirror (also in settings; surfaced here for convenience)
	rt.enableSource = widget.NewCheck("启用源码级魔改", func(on bool) {
		rt.config.RebuildAcknowledged = on
		_ = rt.config.Save()
		rt.refreshEnabled()
	})
	rt.enableSource.SetChecked(rt.config.RebuildAcknowledged)

	rt.mirrorEntry = widget.NewEntry()
	rt.mirrorEntry.SetPlaceHolder(rebuild.DefaultDockerHubMirror)
	if rt.config.RebuildDockerMirror != "" {
		rt.mirrorEntry.SetText(rt.config.RebuildDockerMirror)
	} else {
		rt.mirrorEntry.SetText(rebuild.DefaultDockerHubMirror)
	}
	fillMirror := widget.NewButton("默认源", func() {
		rt.mirrorEntry.SetText(rebuild.DefaultDockerHubMirror)
		rt.pullDirectChk.SetChecked(true)
	})
	rt.pullDirectChk = widget.NewCheck("镜像源直连（不走代理）", nil)
	rt.pullDirectChk.SetChecked(rt.config.RebuildDockerPullDirect)
	rt.archiveChk = widget.NewCheck("完成后 docker save 留档到本地", nil)
	rt.archiveChk.SetChecked(true)

	// ── Step 1 panel: base image ────────────────────────────────────
	rt.bootstrapBtn = widget.NewButton("构建 / 校验基础镜像", rt.startBootstrap)
	rt.bootstrapBtn.Importance = widget.HighImportance
	rt.refreshImgBtn = widget.NewButton("刷新镜像状态", rt.refreshImageStatus)
	rt.checkDepsBtn = widget.NewButton("检查依赖", rt.checkDeps)
	rt.depsLabel = widget.NewLabel("步骤① 只需 Docker + 磁盘；AI/OpenAI 在步骤② 需要。")
	rt.depsLabel.Wrapping = fyne.TextWrapWord

	step1Help := widget.NewRichTextFromMarkdown(
		"**步骤① 基础镜像**\n\n" +
			"- 只装开发工具链（apt / Node 20 / Go 1.24 / NDK r29）\n" +
			"- 本地标签留档，下次直接复用，不重复下载\n" +
			"- 国内默认 Hub 源：`" + rebuild.DefaultDockerHubMirror + "`（可改）\n" +
			"- AI 魔改 / 编译 **不要** 在此步装重依赖",
	)
	step1Help.Wrapping = fyne.TextWrapWord

	rt.step1Panel = container.NewVBox(
		step1Help,
		widget.NewSeparator(),
		widget.NewLabel("Docker Hub 镜像源:"),
		container.NewBorder(nil, nil, nil, fillMirror, rt.mirrorEntry),
		rt.pullDirectChk,
		rt.archiveChk,
		widget.NewLabel("本地镜像名: "+rt.config.RebuildDockerImage),
		widget.NewLabel("留档标签: "+rebuild.BuilderStableImageTag(rt.config.RebuildDockerImage)),
		container.NewHBox(rt.bootstrapBtn, rt.refreshImgBtn, rt.checkDepsBtn),
		rt.imageStatus,
		rt.depsLabel,
	)

	// ── Step 2 panel: develop ───────────────────────────────────────
	rt.versionEntry = widget.NewEntry()
	rt.versionEntry.SetPlaceHolder("官方 tag，如 17.16.4")
	rt.versionEntry.SetText("17.16.4")

	rt.magicEntry = widget.NewEntry()
	if rt.config.MagicName != "" {
		rt.magicEntry.SetText(rt.config.MagicName)
	} else {
		rt.magicEntry.SetText("frida")
	}
	randomBtn := widget.NewButton("随机", func() {
		rt.magicEntry.SetText(utils.GenerateRandomName())
	})

	rt.portEntry = widget.NewEntry()
	rt.portEntry.SetPlaceHolder("官方 27042；改了才动 DEFAULT_CONTROL_PORT")
	if rt.config.DefaultPort > 0 {
		rt.portEntry.SetText(strconv.Itoa(rt.config.DefaultPort))
	} else {
		rt.portEntry.SetText("27042")
	}

	targetsBox := container.NewVBox()
	for _, tgt := range rebuild.SupportedBuildTargets() {
		t := tgt
		label := t.Label
		if !t.DockerFriendly {
			label += " ⚠"
		}
		chk := widget.NewCheck(label, nil)
		if t.ID == "android-arm64" {
			chk.SetChecked(true)
		}
		rt.targetChecks[t.ID] = chk
		targetsBox.Add(chk)
	}
	targetsScroll := container.NewVScroll(targetsBox)
	targetsScroll.SetMinSize(fyne.NewSize(0, 140))

	rt.useGrokCheck = widget.NewCheck("使用本机 grok（端点跟 GUI）", nil)
	rt.useGrokCheck.SetChecked(rt.config.RebuildUseLocalGrok)
	if _, ok := rebuild.ResolveGrokBinary(nil); ok {
		rt.useGrokCheck.SetText("本机已有 grok — 勾选使用")
	} else {
		rt.useGrokCheck.SetText("未检测到 grok（将用朴素替换）")
	}
	rt.useGUIProxyCheck = widget.NewCheck("OpenAI 端点走 GUI 代理出口（默认不走）", func(on bool) {
		rt.config.RebuildAgentUseGUIProxy = on
		_ = rt.config.Save()
	})
	rt.useGUIProxyCheck.SetChecked(rt.config.RebuildAgentUseGUIProxy)

	rt.profileSelect = widget.NewSelect([]string{"safe", "deep", "abi", "explore"}, nil)
	rt.profileSelect.SetSelected("deep") // default: server+client protocol/API/idents sync
	rt.randomAgentChk = widget.NewCheck("随机 agent 落盘前缀（可选；种子=magic+构建号）", nil)
	profileHelp := widget.NewLabel("deep（推荐）：服务端+客户端同步 re.frida.* / /re/frida/ / frida:rpc；导出去符号 + 注入 TU 花指令。abi：仅 gum injector / agent / 进程枚举白名单改标识符。须用 catalog 内 wheel + PROTOCOL-SYNC。")
	profileHelp.Wrapping = fyne.TextWrapWord
	profileHelp.Importance = widget.LowImportance

	rt.developBtn = widget.NewButton("启动步骤② 魔改+编译", rt.startDevelop)
	rt.developBtn.Importance = widget.HighImportance
	rt.fullOneShotBtn = widget.NewButton("一键深度定制（①+② deep）", rt.startFull)

	step2Help := widget.NewLabel("步骤②：在已就绪的基础镜像上魔改源码并编译。默认 deep：服务端+客户端协议面同步；产物含 binaries + python/host wheels + PROTOCOL-SYNC。")
	step2Help.Wrapping = fyne.TextWrapWord

	rt.step2Panel = container.NewVBox(
		step2Help,
		widget.NewSeparator(),
		widget.NewLabel("Frida 版本 (depth=1):"),
		rt.versionEntry,
		container.NewBorder(nil, nil, widget.NewLabel("魔改名"), randomBtn, rt.magicEntry),
		container.NewBorder(nil, nil, widget.NewLabel("端口"), nil, rt.portEntry),
		widget.NewLabel("编译目标:"),
		targetsScroll,
		container.NewBorder(nil, nil, widget.NewLabel("魔改强度"), nil, rt.profileSelect),
		profileHelp,
		rt.randomAgentChk,
		rt.useGUIProxyCheck,
		rt.useGrokCheck,
		container.NewHBox(rt.developBtn, rt.fullOneShotBtn),
	)

	// work area stacks step panels
	rt.workStack = container.NewStack(rt.step1Panel, rt.step2Panel)
	rt.step2Panel.Hide()

	// ── Agent + log column (fixed visible) ──────────────────────────
	rt.goalsEntry = widget.NewMultiLineEntry()
	rt.goalsEntry.SetPlaceHolder("对 Agent 说：魔改目标 / 约束 / 失败后如何修…（始终显示在此栏，不会滚到屏幕外）")
	rt.goalsEntry.SetMinRowsVisible(4)

	rt.agentLog = NewLogView()
	rt.agentLog.SetMinHeight(160)
	rt.agentLog.SetText("【Agent 对话】计划与文件操作会出现在这里。\n")

	rt.jobLog = NewLogView()
	rt.jobLog.SetMinHeight(120)

	rt.progressBar = widget.NewProgressBar()
	rt.progressLabel = widget.NewLabel("空闲")
	rt.artifactLabel = widget.NewLabel("产物: （完成后显示）")
	rt.artifactLabel.Wrapping = fyne.TextWrapWord

	rt.cancelBtn = widget.NewButton("终止", rt.cancelJob)
	rt.resetBtn = widget.NewButton("重来", rt.resetJob)
	rt.openArtsBtn = widget.NewButton("产物目录", rt.openArtifacts)

	agentHeader := widget.NewLabel("Agent 对话")
	agentHeader.TextStyle = fyne.TextStyle{Bold: true}
	logHeader := widget.NewLabel("任务日志")
	logHeader.TextStyle = fyne.TextStyle{Bold: true}

	agentPanel := container.NewBorder(
		container.NewVBox(
			agentHeader,
			widget.NewLabel("魔改目标输入:"),
			rt.goalsEntry,
			container.NewHBox(rt.cancelBtn, rt.resetBtn, rt.openArtsBtn),
			rt.progressBar,
			rt.progressLabel,
			rt.artifactLabel,
			widget.NewSeparator(),
			logHeader,
		),
		nil, nil, nil,
		container.NewVSplit(
			rt.agentLog.CanvasObject(),
			rt.jobLog.CanvasObject(),
		),
	)

	// ── Left nav column ────────────────────────────────────────────
	hostHint := widget.NewLabel("Host " + rebuild.HostPlatformLabel() + "\n编译仅 Docker/Linux")
	hostHint.Wrapping = fyne.TextWrapWord
	hostHint.Importance = widget.MediumImportance

	// Catalog browser
	rt.catalogStatus = widget.NewLabel("产物库: 加载中…")
	rt.catalogStatus.Wrapping = fyne.TextWrapWord
	rt.installHint = widget.NewLabel("选中条目后可打开目录 / 查看 python/*.whl")
	rt.installHint.Wrapping = fyne.TextWrapWord
	rt.catalogSel = -1
	rt.catalogList = widget.NewList(
		func() int { return len(rt.catalogEntries) },
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			if i < 0 || i >= len(rt.catalogEntries) {
				return
			}
			e := rt.catalogEntries[i]
			flags := ""
			if e.HasBin {
				flags += "📦"
			}
			if e.HasWheel {
				flags += "🐍"
			}
			obj.(*widget.Label).SetText(fmt.Sprintf("%s %s / %s / %s", flags, e.Version, e.Platform, e.Magic))
		},
	)
	rt.catalogList.OnSelected = func(id widget.ListItemID) {
		rt.catalogSel = int(id)
		if id >= 0 && id < len(rt.catalogEntries) {
			e := rt.catalogEntries[id]
			rt.installHint.SetText(fmt.Sprintf("路径:\n%s\n\nHost frida wheel（按本机 OS/CPU 选目录）:\n  python/host/windows-amd64|windows-arm64\n  python/host/macos-x86_64|macos-arm64\n  python/host/linux-x86_64|linux-arm64\n\npip install --force-reinstall --no-deps python/host/<平台>/frida-*.whl\npip install --force-reinstall --no-deps python/frida_tools-*.whl\n# 版本须 == %s，与 server magic 一致",
				e.Path, e.Version))
		}
	}
	rt.refreshCatBtn = widget.NewButton("刷新产物库", rt.refreshCatalog)
	rt.openCatRootBtn = widget.NewButton("打开总目录", rt.openCatalogRoot)
	rt.openCatSelBtn = widget.NewButton("打开选中项", rt.openCatalogSelected)
	catScroll := container.NewVScroll(rt.catalogList)
	catScroll.SetMinSize(fyne.NewSize(0, 140))
	catalogBox := container.NewVBox(
		widget.NewLabelWithStyle("历史产物库", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("version / platform / magic"),
		rt.catalogStatus,
		catScroll,
		container.NewHBox(rt.refreshCatBtn, rt.openCatRootBtn),
		rt.openCatSelBtn,
		rt.installHint,
	)

	// Nav uses separate buttons (Fyne objects cannot be parented twice)
	navStep1 := widget.NewButton("① 基础镜像", func() { rt.selectStep(0) })
	navStep2 := widget.NewButton("② 魔改开发", func() { rt.selectStep(1) })
	nav := container.NewVBox(
		widget.NewLabelWithStyle("源码魔改", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		rt.enableSource,
		widget.NewSeparator(),
		widget.NewLabel("流程步骤"),
		navStep1,
		navStep2,
		widget.NewSeparator(),
		hostHint,
		widget.NewSeparator(),
		catalogBox,
	)
	navScroll := container.NewVScroll(nav)
	navScroll.SetMinSize(fyne.NewSize(220, 0))

	// ── Center workspace ───────────────────────────────────────────
	workCard := widget.NewCard("", "", container.NewVScroll(rt.workStack))

	// ── Three-column layout ────────────────────────────────────────
	centerRight := container.NewHSplit(workCard, agentPanel)
	centerRight.Offset = 0.48

	mainSplit := container.NewHSplit(navScroll, centerRight)
	mainSplit.Offset = 0.18

	topBar := container.NewBorder(nil, nil,
		container.NewHBox(rt.step1Btn, widget.NewLabel("→"), rt.step2Btn),
		nil,
		rt.stepBadge,
	)

	rt.content = container.NewBorder(
		container.NewVBox(topBar, widget.NewSeparator()),
		nil, nil, nil,
		mainSplit,
	)

	rt.refreshEnabled()
	rt.selectStep(0)
	// async image status + catalog
	go rt.refreshImageStatus()
	rt.refreshCatalog()
}

func (rt *RebuildTab) selectStep(i int) {
	rt.stepIndex = i
	if i == 0 {
		rt.step1Panel.Show()
		rt.step2Panel.Hide()
		rt.stepBadge.SetText("当前：步骤① 基础镜像（工具链）")
		rt.step1Btn.Importance = widget.HighImportance
		rt.step2Btn.Importance = widget.MediumImportance
	} else {
		rt.step1Panel.Hide()
		rt.step2Panel.Show()
		rt.stepBadge.SetText("当前：步骤② 魔改开发（AI + 编译）")
		rt.step1Btn.Importance = widget.MediumImportance
		rt.step2Btn.Importance = widget.HighImportance
	}
	rt.step1Btn.Refresh()
	rt.step2Btn.Refresh()
	rt.workStack.Refresh()
}

func (rt *RebuildTab) refreshEnabled() {
	on := rt.enableSource != nil && rt.enableSource.Checked
	set := func(b *widget.Button, enable bool) {
		if b == nil {
			return
		}
		if enable && !rt.running {
			b.Enable()
		} else if !enable {
			b.Disable()
		}
	}
	set(rt.bootstrapBtn, on)
	set(rt.developBtn, on)
	set(rt.fullOneShotBtn, on)
}

func (rt *RebuildTab) selectedTargets() []string {
	var ids []string
	for id, chk := range rt.targetChecks {
		if chk.Checked {
			ids = append(ids, id)
		}
	}
	return ids
}

func (rt *RebuildTab) saveMirrorConfig() {
	if rt.mirrorEntry != nil {
		rt.config.RebuildDockerMirror = strings.TrimSpace(rt.mirrorEntry.Text)
	}
	if rt.pullDirectChk != nil {
		rt.config.RebuildDockerPullDirect = rt.pullDirectChk.Checked
	}
	if rt.config.RebuildDockerMirror == "" {
		rt.config.RebuildDockerMirror = rebuild.DefaultDockerHubMirror
	}
	_ = rt.config.Save()
}

func (rt *RebuildTab) baseJobConfig(mode rebuild.JobMode) rebuild.JobConfig {
	rt.saveMirrorConfig()
	work := filepath.Join(rt.config.WorkDir, "rebuild")
	arts := rebuild.DefaultArtifactDir(rt.config.WorkDir)
	useGUIProxy := false
	if rt.useGUIProxyCheck != nil {
		useGUIProxy = rt.useGUIProxyCheck.Checked
	}
	version := "17.16.4"
	if rt.versionEntry != nil {
		version = strings.TrimSpace(rt.versionEntry.Text)
	}
	magic := "frida"
	if rt.magicEntry != nil {
		magic = strings.TrimSpace(rt.magicEntry.Text)
	}
	port := 27042
	if rt.portEntry != nil {
		if p, err := strconv.Atoi(strings.TrimSpace(rt.portEntry.Text)); err == nil && p > 0 {
			port = p
		}
	}
	goals := ""
	if rt.goalsEntry != nil {
		goals = rt.goalsEntry.Text
	}
	archive := rt.archiveChk != nil && rt.archiveChk.Checked
	useGrok := rt.useGrokCheck != nil && rt.useGrokCheck.Checked
	profile := "deep"
	if rt.profileSelect != nil && strings.TrimSpace(rt.profileSelect.Selected) != "" {
		profile = strings.TrimSpace(rt.profileSelect.Selected)
	}
	dirFile := filepath.Join(work, "fridare-directions.json")
	return rebuild.JobConfig{
		FridaVersion:     version,
		TargetIDs:        rt.selectedTargets(),
		MagicName:        magic,
		ListenPort:       port,
		Goals:            goals,
		WorkDir:          work,
		ArtifactDir:      arts,
		Proxy:            rt.config.Proxy,
		OpenAIBaseURL:    rt.config.OpenAIBaseURL,
		OpenAIAPIKey:     rt.config.OpenAIAPIKey,
		OpenAIModel:      rt.config.OpenAIModel,
		DockerImage:      rt.config.RebuildDockerImage,
		DockerMirror:     rt.config.RebuildDockerMirror,
		DockerPullDirect: rt.config.RebuildDockerPullDirect,
		UseExistingGrok:  useGrok,
		AgentUseGUIProxy: useGUIProxy,
		MinDiskGB:        rt.config.RebuildMinDiskGB,
		Mode:             mode,
		ArchiveImage:     archive,
		DryRun:           false,
		DirectionProfile:  profile,
		StripSymbols:      profile != "safe",
		RandomAgentPrefix: rt.randomAgentChk != nil && rt.randomAgentChk.Checked,
		DirectionFile:    dirFile,
	}
}

func (rt *RebuildTab) refreshImageStatus() {
	rt.doRefreshImageStatus()
}

func (rt *RebuildTab) doRefreshImageStatus() {
	img := rt.config.RebuildDockerImage
	if img == "" {
		img = "fridare/frida-builder:latest"
	}
	mirror := rt.config.RebuildDockerMirror
	if mirror == "" {
		mirror = rebuild.DefaultDockerHubMirror
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		ready, detail := rebuild.ProbeLocalBuilderImage(ctx, rebuild.ExecRunner{}, img)
		text := "镜像状态: " + detail
		if ready {
			text = "✓ " + text
		} else {
			text = "○ " + text
		}
		arch := rebuild.ImageArchiveDir(filepath.Join(rt.config.WorkDir, "rebuild"))
		text += "\n档案目录: " + arch
		text += "\n镜像源: " + mirror
		fyne.Do(func() {
			if rt.imageStatus != nil {
				rt.imageStatus.SetText(text)
			}
		})
	}()
}

func (rt *RebuildTab) checkDeps() {
	rt.appendLog("INFO: 开始依赖探测…")
	go func() {
		report := rebuild.ProbeDeps(rebuild.ProbeOptions{
			WorkDir:       rt.config.WorkDir,
			Proxy:         rt.config.Proxy,
			OpenAIBaseURL: rt.config.OpenAIBaseURL,
			OpenAIAPIKey:  rt.config.OpenAIAPIKey,
			MinDiskGB:     rt.config.RebuildMinDiskGB,
		})
		rt.mu.Lock()
		rt.lastDeps = report
		rt.mu.Unlock()

		var b strings.Builder
		b.WriteString(fmt.Sprintf("检查时间: %s\n", report.CheckedAt.Format(time.RFC3339)))
		writeProbe := func(p rebuild.ProbeResult) {
			mark := "✗"
			if p.Available {
				mark = "✓"
			}
			req := ""
			if p.Required {
				req = " [必需]"
			}
			b.WriteString(fmt.Sprintf("%s %s%s: %s\n", mark, p.Name, req, p.Detail))
		}
		writeProbe(report.Docker)
		writeProbe(report.GrokBuild)
		writeProbe(report.Git)
		writeProbe(report.Proxy)
		writeProbe(report.Disk)
		writeProbe(report.OpenAI)
		if report.Ready {
			b.WriteString("\n步骤② 就绪。")
		} else {
			b.WriteString("\n步骤② 未完全就绪（步骤① 仍可只检查 Docker/磁盘）。\n")
			b.WriteString("OpenAI 推荐: " + rebuild.RecommendedAPIBase + " QQ " + rebuild.RecommendedQQGroup + "\n")
		}
		text := b.String()
		fyne.Do(func() {
			rt.depsLabel.SetText(text)
			rt.updateStatus("依赖检查完成")
		})
		rt.appendLog("INFO: 依赖检查完成 ready=" + strconv.FormatBool(report.Ready))
		rt.appendAgent("[deps]\n" + text)
	}()
}

func (rt *RebuildTab) startBootstrap() {
	if !rt.gateEnabled() {
		return
	}
	rt.saveMirrorConfig()
	cfg := rt.baseJobConfig(rebuild.JobModeBootstrap)
	// bootstrap doesn't need magic validation hard-fail if empty
	msg := fmt.Sprintf("步骤① 构建基础镜像\n镜像: %s\n源: %s 直连=%v\n留档 tar=%v\n\n仅安装工具链，不做 Frida 编译。",
		cfg.DockerImage, cfg.DockerMirror, cfg.DockerPullDirect, cfg.ArchiveImage)
	dialog.ShowConfirm("步骤① 基础镜像", msg, func(ok bool) {
		if !ok {
			return
		}
		rt.selectStep(0)
		rt.appendAgent("→ 开始步骤①：构建/校验基础镜像…")
		rt.doStart(cfg)
	}, rt.window)
}

func (rt *RebuildTab) startDevelop() {
	if !rt.gateEnabled() {
		return
	}
	if err := rt.validateDevelopForm(); err != nil {
		dialog.ShowError(err, rt.window)
		return
	}
	cfg := rt.baseJobConfig(rebuild.JobModeDevelop)
	msg := fmt.Sprintf("步骤② 魔改开发\n版本 %s\n目标 %s\n\n使用已就绪基础镜像；AI 在容器工作区魔改并编译。",
		cfg.FridaVersion, strings.Join(cfg.TargetIDs, ", "))
	dialog.ShowConfirm("步骤② 魔改开发", msg, func(ok bool) {
		if !ok {
			return
		}
		rt.selectStep(1)
		rt.appendAgent("→ 开始步骤②：clone → AI 魔改 → Docker 编译…\n目标: " + cfg.Goals)
		rt.doStart(cfg)
	}, rt.window)
}

func (rt *RebuildTab) startFull() {
	if !rt.gateEnabled() {
		return
	}
	if err := rt.validateDevelopForm(); err != nil {
		dialog.ShowError(err, rt.window)
		return
	}
	cfg := rt.baseJobConfig(rebuild.JobModeFull)
	msg := fmt.Sprintf("一键全流程（① 镜像 + ② 魔改编译）\n版本 %s\n目标 %s\n产物 %s",
		cfg.FridaVersion, strings.Join(cfg.TargetIDs, ", "), cfg.ArtifactDir)
	dialog.ShowConfirm("确认全流程", msg, func(ok bool) {
		if !ok {
			return
		}
		rt.appendAgent("→ 一键全流程启动…")
		rt.doStart(cfg)
	}, rt.window)
}

func (rt *RebuildTab) gateEnabled() bool {
	if rt.enableSource == nil || !rt.enableSource.Checked {
		dialog.ShowInformation("未启用", "请先勾选「启用源码级魔改」。", rt.window)
		return false
	}
	return true
}

func (rt *RebuildTab) validateDevelopForm() error {
	if err := rebuild.ProxyRequiredForSourceMod(rt.config.Proxy); err != nil {
		return err
	}
	agentProxyCfg := rebuild.JobConfig{
		Proxy:            rt.config.Proxy,
		AgentUseGUIProxy: rt.useGUIProxyCheck != nil && rt.useGUIProxyCheck.Checked,
	}
	if err := rebuild.RequireGUIProxyForAgent(agentProxyCfg); err != nil {
		return err
	}
	magic := strings.TrimSpace(rt.magicEntry.Text)
	if err := core.ValidateMagicName(magic); err != nil {
		return err
	}
	if strings.TrimSpace(rt.versionEntry.Text) == "" {
		return fmt.Errorf("请填写 Frida 官方版本")
	}
	if len(rt.selectedTargets()) == 0 {
		return fmt.Errorf("请至少选择一个编译目标")
	}
	report := rebuild.ProbeDeps(rebuild.ProbeOptions{
		WorkDir:       rt.config.WorkDir,
		Proxy:         rt.config.Proxy,
		OpenAIBaseURL: rt.config.OpenAIBaseURL,
		OpenAIAPIKey:  rt.config.OpenAIAPIKey,
		MinDiskGB:     rt.config.RebuildMinDiskGB,
	})
	rt.lastDeps = report
	if err := rebuild.CanStartSourceJob(report, rt.config.Proxy); err != nil {
		return fmt.Errorf("%w\n\n%s", err, rebuild.RecommendedEndpointHelp)
	}
	return nil
}

func (rt *RebuildTab) doStart(cfg rebuild.JobConfig) {
	rt.mu.Lock()
	rt.running = true
	rt.mu.Unlock()
	fyne.Do(func() {
		if rt.bootstrapBtn != nil {
			rt.bootstrapBtn.Disable()
		}
		if rt.developBtn != nil {
			rt.developBtn.Disable()
		}
		if rt.fullOneShotBtn != nil {
			rt.fullOneShotBtn.Disable()
		}
		rt.progressBar.SetValue(0)
		rt.progressLabel.SetText("启动中…")
	})
	rt.appendLog(fmt.Sprintf("INFO: 任务启动 mode=%s version=%s", cfg.Mode, cfg.FridaVersion))
	rt.appendAgent(fmt.Sprintf("[%s] mode=%s version=%s targets=%v",
		time.Now().Format("15:04:05"), cfg.Mode, cfg.FridaVersion, cfg.TargetIDs))

	err := rt.orch.Start(cfg, func(ev rebuild.ProgressEvent) {
		fyne.Do(func() {
			rt.progressBar.SetValue(ev.Percent)
			rt.progressLabel.SetText(fmt.Sprintf("[%s] %s", ev.Stage, ev.Message))
		})
		line := fmt.Sprintf("%s [%s] %s", ev.Time.Format("15:04:05"), ev.Stage, ev.Message)
		if ev.Err != "" {
			line += " ERR=" + ev.Err
		}
		rt.appendLog(line)
		// Agent-visible stages
		switch ev.Stage {
		case rebuild.StageModPlan, rebuild.StageModApply, rebuild.StageBootstrap, rebuild.StageBuild, rebuild.StageDone, rebuild.StageFailed:
			rt.appendAgent(line)
		}
		if ev.Stage == rebuild.StageDone || ev.Stage == rebuild.StageFailed || ev.Stage == rebuild.StageCancelled {
			rt.onJobTerminal(ev)
		}
	})
	if err != nil {
		rt.mu.Lock()
		rt.running = false
		rt.mu.Unlock()
		fyne.Do(func() {
			rt.refreshEnabled()
			dialog.ShowError(err, rt.window)
		})
		rt.appendLog("ERROR: " + err.Error())
		rt.appendAgent("ERROR: " + err.Error())
	}
}

func (rt *RebuildTab) onJobTerminal(ev rebuild.ProgressEvent) {
	rt.mu.Lock()
	rt.running = false
	rt.mu.Unlock()
	st := rt.orch.State()
	fyne.Do(func() {
		rt.refreshEnabled()
		var extra string
		if st.LogPath != "" {
			extra = "\n日志: " + st.LogPath
			if st.LatestLog != "" {
				extra += "\n" + st.LatestLog
			}
		}
		if st.Artifact != "" {
			rt.artifactLabel.SetText("产物: " + st.Artifact + extra)
		} else if st.LogPath != "" {
			rt.artifactLabel.SetText("产物: （未导出）" + extra)
		}
		if ev.Stage == rebuild.StageDone {
			title := "完成"
			body := ev.Message
			if st.Artifact != "" {
				body += "\n\n产物: " + st.Artifact
			}
			body += "\n\n可在左侧「历史产物库」按版本/平台/magic 浏览；python/*.whl 供本机 pip 安装以连接 server。"
			dialog.ShowInformation(title, body, rt.window)
			rt.refreshCatalog()
			// if bootstrap done, offer step 2
			if strings.Contains(ev.Message, "步骤①") || strings.Contains(ev.Message, "基础镜像") {
				rt.selectStep(1)
			}
		}
		if ev.Stage == rebuild.StageFailed && st.LogPath != "" {
			dialog.ShowError(fmt.Errorf("%s\n\n日志:\n%s", ev.Message, st.LogPath), rt.window)
		}
		rt.doRefreshImageStatus()
	})
	rt.updateStatus(string(ev.Stage) + ": " + ev.Message)
}

func (rt *RebuildTab) cancelJob() {
	rt.orch.Cancel()
	rt.appendLog("WARN: 用户请求终止")
	rt.appendAgent("WARN: 用户终止任务")
	rt.updateStatus("正在终止…")
}

func (rt *RebuildTab) resetJob() {
	rt.orch.Reset()
	rt.mu.Lock()
	rt.running = false
	rt.mu.Unlock()
	fyne.Do(func() {
		rt.progressBar.SetValue(0)
		rt.progressLabel.SetText("已重置")
		rt.refreshEnabled()
		rt.artifactLabel.SetText("产物: （完成后显示）")
	})
	rt.appendLog("INFO: 任务已重置")
	rt.appendAgent("INFO: 已重置，可重新开始步骤① 或 ②")
}

func (rt *RebuildTab) openArtifacts() {
	dir := rebuild.DefaultArtifactDir(rt.config.WorkDir)
	st := rt.orch.State()
	if st.Artifact != "" {
		dir = st.Artifact
	}
	_ = os.MkdirAll(dir, 0755)
	openHostFolder(dir)
	rt.appendLog("INFO: 产物目录 " + dir)
	rt.refreshCatalog()
	dialog.ShowInformation("产物目录", dir+"\n\n"+rebuild.ToolsPatchGuidance(rt.magicEntry.Text, rt.config.DefaultPort), rt.window)
}

func (rt *RebuildTab) catalogRootPath() string {
	work := filepath.Join(rt.config.WorkDir, "rebuild")
	return rebuild.CatalogRoot(work)
}

func (rt *RebuildTab) refreshCatalog() {
	root := rt.catalogRootPath()
	go func() {
		ents, err := rebuild.ListCatalog(root)
		fyne.Do(func() {
			if err != nil {
				rt.catalogStatus.SetText("产物库读取失败: " + err.Error())
				return
			}
			rt.catalogEntries = ents
			rt.catalogSel = -1
			if rt.catalogList != nil {
				rt.catalogList.UnselectAll()
				rt.catalogList.Refresh()
			}
			rt.catalogStatus.SetText(fmt.Sprintf("产物库 %d 项\n%s", len(ents), root))
		})
	}()
}

func (rt *RebuildTab) openCatalogRoot() {
	root := rt.catalogRootPath()
	_ = os.MkdirAll(root, 0755)
	openHostFolder(root)
	rt.appendLog("INFO: catalog " + root)
}

func (rt *RebuildTab) openCatalogSelected() {
	if rt.catalogSel < 0 || rt.catalogSel >= len(rt.catalogEntries) {
		dialog.ShowInformation("未选择", "请先在左侧产物库列表中点选 version/platform/magic 条目。", rt.window)
		return
	}
	e := rt.catalogEntries[rt.catalogSel]
	openHostFolder(e.Path)
	rt.appendLog("INFO: catalog entry " + e.Path)
}

// openHostFolder opens a directory in the platform file manager.
func openHostFolder(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	_ = cmd.Start()
	u := "file://" + filepath.ToSlash(dir)
	if parsed, err := url.Parse(u); err == nil && fyne.CurrentApp() != nil {
		_ = fyne.CurrentApp().OpenURL(parsed)
	}
}

func (rt *RebuildTab) appendLog(line string) {
	if rt.addLog != nil {
		rt.addLog(line)
	}
	fyne.Do(func() {
		if rt.jobLog != nil {
			rt.jobLog.Append(line)
		}
	})
}

func (rt *RebuildTab) appendAgent(line string) {
	fyne.Do(func() {
		if rt.agentLog != nil {
			rt.agentLog.Append(line)
		}
	})
}

// Content returns the tab content (fills the tab; no outer scroll needed).
func (rt *RebuildTab) Content() *fyne.Container {
	return rt.content
}

// Refresh reloads config-bound fields.
func (rt *RebuildTab) Refresh() {
	if rt.magicEntry != nil && rt.config.MagicName != "" {
		rt.magicEntry.SetText(rt.config.MagicName)
	}
	if rt.useGrokCheck != nil {
		rt.useGrokCheck.SetChecked(rt.config.RebuildUseLocalGrok)
	}
	if rt.useGUIProxyCheck != nil {
		rt.useGUIProxyCheck.SetChecked(rt.config.RebuildAgentUseGUIProxy)
	}
	if rt.mirrorEntry != nil {
		if rt.config.RebuildDockerMirror != "" {
			rt.mirrorEntry.SetText(rt.config.RebuildDockerMirror)
		} else {
			rt.mirrorEntry.SetText(rebuild.DefaultDockerHubMirror)
		}
	}
	if rt.pullDirectChk != nil {
		rt.pullDirectChk.SetChecked(rt.config.RebuildDockerPullDirect)
	}
	rt.doRefreshImageStatus()
}

// UpdateGlobalConfig syncs toolbar magic/port.
func (rt *RebuildTab) UpdateGlobalConfig(magicName string, port int) {
	if rt.magicEntry != nil && magicName != "" {
		rt.magicEntry.SetText(magicName)
	}
	if rt.portEntry != nil && port > 0 {
		rt.portEntry.SetText(strconv.Itoa(port))
	}
}

