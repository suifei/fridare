package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fridare-gui/internal/config"
	"fridare-gui/internal/i18n"

	"fyne.io/fyne/v2/test"
)

func TestSettingsLanguageSwitchPersistsAndRebuildsWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Cleanup(func() { i18n.SetLanguage(i18n.LangZh) })

	cfg := config.DefaultConfig()
	cfg.NoShowNotice = true
	if cfg.Language != i18n.LangZh {
		t.Fatalf("default language %s", cfg.Language)
	}

	app := test.NewApp()
	mw := NewMainWindow(app, cfg)
	t.Cleanup(func() { mw.window.Close() })

	settingsIdx := -1
	for i, item := range mw.tabContainer.Items {
		if item.Text == "⚙️ 设置" {
			settingsIdx = i
		}
	}
	if settingsIdx < 0 {
		t.Fatal("settings tab missing in Chinese UI")
	}
	// Language card is the first object in the settings content.
	if len(mw.settingsTab.content.Objects) == 0 {
		t.Fatal("settings content empty")
	}
	if mw.settingsTab.langSelect == nil || mw.settingsTab.langSelect.Selected != "中文" {
		t.Fatalf("language select = %#v", mw.settingsTab.langSelect)
	}
	if mw.window.Title() != "Fridare GUI - Frida 魔改工具" {
		t.Fatalf("title %q", mw.window.Title())
	}

	mw.tabContainer.SelectIndex(settingsIdx)
	mw.settingsTab.langSelect.SetSelected("English")

	if i18n.Current() != i18n.LangEn {
		t.Fatalf("runtime language %s", i18n.Current())
	}
	if mw.config.Language != i18n.LangEn {
		t.Fatalf("config language %s", mw.config.Language)
	}
	if mw.window.Title() != "Fridare GUI - Frida patch tool" {
		t.Fatalf("english title %q", mw.window.Title())
	}
	foundSettings := false
	foundDownload := false
	for _, item := range mw.tabContainer.Items {
		if item.Text == "⚙️ Settings" {
			foundSettings = true
		}
		if item.Text == "📥 Download" {
			foundDownload = true
		}
		if item.Text == "⚙️ 设置" || item.Text == "📥 下载" {
			t.Fatalf("chinese tab still visible: %s", item.Text)
		}
	}
	if !foundSettings || !foundDownload {
		t.Fatal("english tabs were not rebuilt")
	}
	if mw.settingsTab.langSelect.Selected != "English" {
		t.Fatalf("select after rebuild %q", mw.settingsTab.langSelect.Selected)
	}
	// The language card stays at the top after the rebuild.
	if len(mw.settingsTab.content.Objects) < 2 {
		t.Fatal("language card should sit above the two-column settings")
	}

	data, err := os.ReadFile(filepath.Join(home, ".config", "fridare", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"language": "en"`) {
		t.Fatalf("config.json missing language:\n%s", data)
	}

	loaded, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Language != i18n.LangEn {
		t.Fatalf("reloaded language %s", loaded.Language)
	}

	mw.settingsTab.langSelect.SetSelected("中文")
	if i18n.Current() != i18n.LangZh || mw.window.Title() != "Fridare GUI - Frida 魔改工具" {
		t.Fatalf("back to chinese: lang=%s title=%q", i18n.Current(), mw.window.Title())
	}
	data, err = os.ReadFile(filepath.Join(home, ".config", "fridare", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"language": "zh"`) {
		t.Fatalf("config did not return to zh:\n%s", data)
	}
}
