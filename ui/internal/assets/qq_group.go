package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// QQ group 555354813 QR — shown in Settings (OpenAI) for trial quota signup.
// Source: https://github.com/suifei/artificer-deck/.../qq-group.png
//
//go:embed qq-group.png
var qqGroupPNG []byte

// QQGroupQR is the embeddable QR image resource for UI display.
var QQGroupQR = &fyne.StaticResource{
	StaticName:    "qq-group.png",
	StaticContent: qqGroupPNG,
}

// QQGroupNumber is the group ID shown next to the QR.
const QQGroupNumber = "555354813"
