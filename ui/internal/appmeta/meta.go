// Package appmeta holds shared product identity for binaries and PE resources.
package appmeta

const (
	// Version is the product version (keep in sync with GUI and release scripts).
	Version = "4.0.7"

	// ProductName is the marketing name.
	ProductName = "Fridare"

	// Company is the vendor string for version resources.
	Company = "suifei"

	// Copyright for version info.
	Copyright = "Copyright (c) suifei. https://github.com/suifei/fridare"

	// GUIAppID is the Fyne application ID.
	GUIAppID = "com.suifei.fridare"

	// GUIAppName is the window / process display name.
	GUIAppName = "Fridare GUI"

	// GUIDescription is the PE FileDescription for the GUI.
	GUIDescription = "Fridare — Frida 魔改与源码重编译工具"

	// CreateDescription is the PE description for fridare-create.
	CreateDescription = "Fridare — iOS DEB 打包工具"

	// PatchDescription is the PE description for fridare-patch.
	PatchDescription = "Fridare — iOS DEB / 二进制补丁工具"

	// ProjectURL is the homepage.
	ProjectURL = "https://github.com/suifei/fridare"
)
