package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fridare-gui/internal/rebuild"
)

func main() {
	work := `D:\works\fridare-rebuild-e2e`
	art := filepath.Join(work, "src", "artifacts", "windows-x86_64")
	cfg := rebuild.JobConfig{
		FridaVersion:     "17.16.4",
		MagicName:        "abcde",
		ListenPort:       27142,
		TargetIDs:        []string{"windows-x86_64"},
		WorkDir:          work,
		DirectionProfile: "deep",
	}
	catalog := rebuild.CatalogRoot(work)
	flat := filepath.Join(work, "artifacts", "windows-x86_64")
	_ = os.MkdirAll(flat, 0755)
	_ = filepath.Walk(art, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		low := strings.ToLower(name)
		if !(strings.Contains(low, "server") || strings.Contains(low, "agent") ||
			strings.Contains(low, "gadget") || strings.Contains(low, "helper") || strings.Contains(low, "inject")) {
			return nil
		}
		if info.Size() < 64*1024 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		return os.WriteFile(filepath.Join(flat, name), b, 0755)
	})
	primary, entries, err := rebuild.OrganizeExportToCatalog(catalog, cfg, art, flat)
	fmt.Println("primary", primary)
	fmt.Println("entries", entries, "err", err)
	for _, e := range entries {
		n, _ := rebuild.PatchArtifactBinaryMarkers(filepath.Join(e, "binaries"), "abcde")
		fmt.Println("binary marker files", e, n)
	}
	server, serr := rebuild.FindMagicServerBinary(primary, "abcde")
	if serr != nil {
		server = filepath.Join(flat, "abcde-server.exe")
	}
	whl, _ := rebuild.FindHostFridaWheel(catalog, "17.16.4", "abcde")
	fmt.Println("server", server)
	fmt.Println("whl", whl)
	r, err := rebuild.CrossCheckServerClientProtocol("abcde", server, whl)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	fmt.Printf("matched=%v serverOK=%v clientOK=%v\nissues=%v\nserver_hits=%v\nclient_hits=%v\n",
		r.Matched, r.ServerOK, r.ClientOK, r.Issues, r.ServerHits, r.ClientHits)
	_ = rebuild.WriteProtocolCrossCheckReport(filepath.Join(primary, "PROTOCOL-SYNC.json"), r)
	scratch := `C:\Users\SuiFei\AppData\Local\Temp\grok-goal-067582e6a16c\implementer`
	_ = os.MkdirAll(scratch, 0755)
	_ = rebuild.WriteProtocolCrossCheckReport(filepath.Join(scratch, "protocol-crosscheck.txt"), r)
	_ = os.WriteFile(filepath.Join(scratch, "win-server-catalog.txt"),
		[]byte(fmt.Sprintf("primary=%s\nserver=%s\nsize=%d\nmatched=%v\n", primary, server, fileSize(server), r.Matched)), 0644)
	if !r.Matched {
		os.Exit(1)
	}
}

func fileSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}
