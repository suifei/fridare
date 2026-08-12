package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// LogView is a high-contrast, monospace log area (not gray markdown code blocks).
type LogView struct {
	rich   *widget.RichText
	scroll *container.Scroll
	text   string
	maxRun int // max runes kept
}

// NewLogView creates a log view with theme foreground color + monospace.
func NewLogView() *LogView {
	rt := widget.NewRichText()
	rt.Wrapping = fyne.TextWrapOff
	lv := &LogView{
		rich:   rt,
		maxRun: 120_000,
	}
	lv.scroll = container.NewScroll(rt)
	lv.scroll.SetMinSize(fyne.NewSize(0, 72))
	lv.SetText("")
	return lv
}

// CanvasObject returns the scrollable log widget.
func (l *LogView) CanvasObject() fyne.CanvasObject {
	return l.scroll
}

// SetMinHeight sets the preferred min height of the scroll area.
func (l *LogView) SetMinHeight(h float32) {
	l.scroll.SetMinSize(fyne.NewSize(0, h))
}

// Text returns current log content.
func (l *LogView) Text() string {
	return l.text
}

// SetText replaces log content.
func (l *LogView) SetText(s string) {
	l.text = s
	l.refresh()
}

// Append adds a line (or block) to the log.
func (l *LogView) Append(s string) {
	if s == "" {
		return
	}
	if l.text != "" && !strings.HasSuffix(l.text, "\n") {
		l.text += "\n"
	}
	l.text += s
	if !strings.HasSuffix(l.text, "\n") {
		l.text += "\n"
	}
	// cap size
	if len(l.text) > l.maxRun {
		l.text = l.text[len(l.text)-l.maxRun*3/4:]
		if i := strings.IndexByte(l.text, '\n'); i >= 0 {
			l.text = l.text[i+1:]
		}
	}
	l.refresh()
	// scroll to end
	l.scroll.ScrollToBottom()
}

func (l *LogView) refresh() {
	body := l.text
	if strings.TrimSpace(body) == "" {
		body = "日志输出（等宽高对比）\n"
	}
	// Use TextSegment with Foreground + monospaced caption size — readable on light theme
	l.rich.Segments = []widget.RichTextSegment{
		&widget.TextSegment{
			Text: body,
			Style: widget.RichTextStyle{
				Alignment: fyne.TextAlignLeading,
				Inline:    false,
				SizeName:  theme.SizeNameCaptionText,
				TextStyle: fyne.TextStyle{Monospace: true},
				ColorName: theme.ColorNameForeground,
			},
		},
	}
	l.rich.Refresh()
}

// --- legacy LogEntry API used by MainWindow ---

// LogEntry wraps LogView for main window bottom log.
type LogEntry struct {
	view *LogView
}

// NewLogEntry creates the main window log.
func NewLogEntry() *LogEntry {
	return &LogEntry{view: NewLogView()}
}

func (l *LogEntry) updateContent() {
	l.view.refresh()
}

// SetLogText sets full log text.
func (l *LogEntry) SetLogText(text string) {
	l.view.SetText(text)
}

// AppendLogText appends text.
func (l *LogEntry) AppendLogText(text string) {
	l.view.Append(text)
}

// String returns content.
func (l *LogEntry) String() string {
	return l.view.Text()
}

// CanvasObject for layout.
func (l *LogEntry) CanvasObject() fyne.CanvasObject {
	return l.view.CanvasObject()
}

// SetMinHeight forwards to view.
func (l *LogEntry) SetMinHeight(h float32) {
	l.view.SetMinHeight(h)
}
