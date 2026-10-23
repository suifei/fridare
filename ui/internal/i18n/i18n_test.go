package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestDefaultIsChinese(t *testing.T) {
	SetLanguage("")
	if Current() != LangZh {
		t.Fatalf("current %s", Current())
	}
	got := T("下载")
	if got != "下载" {
		t.Fatalf("default should keep Chinese, got %q", got)
	}
}

func TestEnglishSwitchAndFallback(t *testing.T) {
	t.Cleanup(func() { SetLanguage(LangZh) })
	SetLanguage("EN")
	if Current() != LangEn {
		t.Fatal(Current())
	}
	if LabelFor("en") != LabelEn || LabelFor("") != LabelZh || LabelFor("zh") != LabelZh {
		t.Fatal("labels")
	}
	if CodeFromLabel(LabelEn) != LangEn || CodeFromLabel(LabelZh) != LangZh {
		t.Fatal("codes")
	}
	// Unknown key falls back to the Chinese source.
	if T("这个键不在目录里") != "这个键不在目录里" {
		t.Fatal("fallback")
	}
	SetLanguage("nope")
	if Current() != LangZh {
		t.Fatal("invalid language must be zh")
	}
}

func TestTfKeepsVerbs(t *testing.T) {
	SetLanguage(LangZh)
	got := Tf("端口: %d", 27042)
	if got != "端口: 27042" {
		t.Fatal(got)
	}
	if !strings.Contains(fmt.Sprintf("%s", LangEn), "en") {
		t.Fatal("sanity")
	}
}
