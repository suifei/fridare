package ui

import (
	"testing"

	"fyne.io/fyne/v2/theme"
)

func TestFridareTheme_TextScale(t *testing.T) {
	for _, mode := range []string{"light", "dark", "auto"} {
		th := NewFridareTheme(mode)
		body := th.Size(theme.SizeNameText)
		head := th.Size(theme.SizeNameHeadingText)
		sub := th.Size(theme.SizeNameSubHeadingText)
		cap := th.Size(theme.SizeNameCaptionText)
		// Compact body for dense GUI (≈12–14)
		if body < 12 || body > 14 {
			t.Fatalf("%s body size %v out of expected range", mode, body)
		}
		if head <= body {
			t.Fatalf("%s heading %v should be > body %v", mode, head, body)
		}
		if sub <= body {
			t.Fatalf("%s subheading %v should be > body %v", mode, sub, body)
		}
		if cap >= body {
			t.Fatalf("%s caption %v should be < body %v", mode, cap, body)
		}
		// hierarchy: caption < body < sub < heading
		if !(cap < body && body < sub && sub < head) {
			t.Fatalf("%s scale broken: cap=%v body=%v sub=%v head=%v", mode, cap, body, sub, head)
		}
	}
}

func TestFridareTheme_PaddingPositive(t *testing.T) {
	th := NewFridareTheme("light")
	if th.Size(theme.SizeNamePadding) < 4 {
		t.Fatal("padding too small")
	}
	if th.Size(theme.SizeNameInnerPadding) < 6 {
		t.Fatal("inner padding too small")
	}
}
