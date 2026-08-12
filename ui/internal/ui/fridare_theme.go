package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// FridareTheme wraps light/dark base themes with compact typography.
// Goal: clear hierarchy without oversized text or cramped padding.
type FridareTheme struct {
	base fyne.Theme
}

// NewFridareTheme returns a themed variant.
// mode: "light", "dark", or "auto"/"" (system default base).
func NewFridareTheme(mode string) fyne.Theme {
	var base fyne.Theme
	switch mode {
	case "dark":
		base = theme.DarkTheme()
	case "light":
		base = theme.LightTheme()
	default:
		base = theme.DefaultTheme()
	}
	return &FridareTheme{base: base}
}

func (t *FridareTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Improve log / secondary text contrast: disabled was too gray to read
	if name == theme.ColorNameDisabled {
		// Use a softened foreground instead of near-invisible gray
		fg := t.base.Color(theme.ColorNameForeground, variant)
		if rgba, ok := fg.(color.NRGBA); ok {
			return color.NRGBA{R: rgba.R, G: rgba.G, B: rgba.B, A: 180}
		}
		if rgba, ok := fg.(color.RGBA); ok {
			return color.NRGBA{R: rgba.R, G: rgba.G, B: rgba.B, A: 180}
		}
	}
	// Placeholders slightly stronger than stock
	if name == theme.ColorNamePlaceHolder {
		fg := t.base.Color(theme.ColorNameForeground, variant)
		if rgba, ok := toNRGBA(fg); ok {
			return color.NRGBA{R: rgba.R, G: rgba.G, B: rgba.B, A: 140}
		}
	}
	return t.base.Color(name, variant)
}

func toNRGBA(c color.Color) (color.NRGBA, bool) {
	switch v := c.(type) {
	case color.NRGBA:
		return v, true
	case color.RGBA:
		return color.NRGBA{R: v.R, G: v.G, B: v.B, A: v.A}, true
	default:
		r, g, b, a := c.RGBA()
		return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}, true
	}
}

func (t *FridareTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *FridareTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

// Size: compact scale — body ~13, avoid oversized headings that waste chrome.
func (t *FridareTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius:
		return 3
	case theme.SizeNameSelectionRadius:
		return 3
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameLineSpacing:
		return 2
	default:
		return t.base.Size(name)
	}
}
