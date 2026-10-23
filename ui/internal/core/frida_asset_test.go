package core

import "testing"

func TestFindAsset_ToolsWheelIsNotNativeFridaWheel(t *testing.T) {
	fc := &FridaClient{}
	ver := &FridaVersion{
		Version: "17.17.0",
		Assets: []Asset{
			{Name: "frida-17.17.0-cp37-abi3-linux_x86_64.whl"},
			{Name: "frida_tools-14.10.4-py3-none-any.whl"},
			{Name: "frida-tools-14.10.4-py3-none-any.whl"},
		},
	}
	got, err := fc.FindAsset(ver, Platform{}, FileTypeTools)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "frida_tools-14.10.4-py3-none-any.whl" {
		t.Fatalf("picked %s; native frida wheel must not match the tools package", got.Name)
	}
}
