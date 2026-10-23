package ui

import "testing"

func TestParseAssetInfo_DistinguishesToolsFromNativeWheel(t *testing.T) {
	dt := &DownloadTab{}
	cases := []struct {
		name     string
		fileType string
	}{
		{"frida-server-17.17.0-android-arm64.xz", "frida-server"},
		{"frida-17.17.0-cp37-abi3-win_amd64.whl", "frida"},
		{"frida_tools-14.10.4-py3-none-any.whl", "frida-tools"},
		{"frida-tools-14.10.4.tar.gz", "frida-tools"},
	}
	for _, c := range cases {
		_, ft := dt.parseAssetInfo(c.name)
		if ft != c.fileType {
			t.Fatalf("%s type %q want %q", c.name, ft, c.fileType)
		}
	}
}
