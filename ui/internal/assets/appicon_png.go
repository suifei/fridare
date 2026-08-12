package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// AppIconPNG is a PNG window/taskbar icon (Windows title-bar requires PNG, not ICO-as-bytes).
//
//go:embed appicon.png
var appIconPNGBytes []byte

// AppIconPNG resource for fyne.Window.SetIcon / App.SetIcon.
var AppIconPNG = &fyne.StaticResource{
	StaticName:    "appicon.png",
	StaticContent: appIconPNGBytes,
}
