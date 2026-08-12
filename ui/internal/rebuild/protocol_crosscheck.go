package rebuild

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProtocolCrossCheckResult compares server binary vs client wheel protocol surface.
type ProtocolCrossCheckResult struct {
	Magic           string   `json:"magic"`
	ServerPath      string   `json:"server_path"`
	ClientWheelPath string   `json:"client_wheel_path,omitempty"`
	ServerOK        bool     `json:"server_ok"`
	ClientOK        bool     `json:"client_ok"`
	Matched         bool     `json:"matched"`
	Issues          []string `json:"issues,omitempty"`
	ServerHits      map[string]int `json:"server_hits"`
	ClientHits      map[string]int `json:"client_hits,omitempty"`
}

// CrossCheckServerClientProtocol verifies deep-mod protocol alignment:
// - server must contain re.{magic}. and {magic}:rpc (or same-length product markers)
// - server must not retain re.frida. / frida:rpc as exclusive channel
// - if clientWheel is non-empty, same expectations on wheel contents
func CrossCheckServerClientProtocol(magic, serverPath, clientWheelPath string) (ProtocolCrossCheckResult, error) {
	r := ProtocolCrossCheckResult{
		Magic: magic, ServerPath: serverPath, ClientWheelPath: clientWheelPath,
		ServerHits: map[string]int{}, ClientHits: map[string]int{},
	}
	if len(magic) != 5 {
		return r, fmt.Errorf("magic must be 5 letters")
	}
	pas := strings.ToUpper(magic[:1]) + magic[1:]
	wantProto := "re." + magic + "."
	wantPath := "/re/" + magic + "/"
	wantRPC := magic + ":rpc"
	oldProto := "re.frida."
	oldPath := "/re/frida/"
	oldRPC := "frida:rpc"

	sb, err := os.ReadFile(serverPath)
	if err != nil {
		return r, err
	}
	r.ServerHits[wantProto] = bytes.Count(sb, []byte(wantProto))
	r.ServerHits[wantPath] = bytes.Count(sb, []byte(wantPath))
	r.ServerHits[wantRPC] = bytes.Count(sb, []byte(wantRPC))
	r.ServerHits[oldProto] = bytes.Count(sb, []byte(oldProto))
	r.ServerHits[oldPath] = bytes.Count(sb, []byte(oldPath))
	r.ServerHits[oldRPC] = bytes.Count(sb, []byte(oldRPC))

	if r.ServerHits[wantRPC] == 0 && r.ServerHits[wantProto] == 0 {
		// product basename markers still count as partial
		if bytes.Contains(sb, []byte(magic+"-server")) || bytes.Contains(sb, []byte(magic+"_server")) {
			r.ServerHits[magic+"-server"] = bytes.Count(sb, []byte(magic+"-server"))
		} else {
			r.Issues = append(r.Issues, "server missing magic rpc/protocol markers")
		}
	}
	if r.ServerHits[oldRPC] > 0 && r.ServerHits[wantRPC] == 0 {
		r.Issues = append(r.Issues, "server still has frida:rpc without magic:rpc")
	}
	if r.ServerHits[oldProto] > 0 && r.ServerHits[wantProto] == 0 {
		r.Issues = append(r.Issues, "server still has re.frida. without re."+magic+".")
	}
	if r.ServerHits[oldPath] > 0 && r.ServerHits[wantPath] == 0 {
		r.Issues = append(r.Issues, "server still has /re/frida/ object path without /re/"+magic+"/")
	}
	if r.ServerHits[wantPath] == 0 && r.ServerHits[wantProto] > 0 {
		// interfaces rewritten but object paths missing → DBus UNKNOWN_METHOD
		r.Issues = append(r.Issues, "server missing /re/"+magic+"/ object-path markers (HostSession path)")
	}
	r.ServerOK = len(r.Issues) == 0

	if clientWheelPath != "" {
		ch, err := countInZipOrFile(clientWheelPath, []string{wantProto, wantPath, wantRPC, oldProto, oldPath, oldRPC, "Frida.", pas + "."})
		if err != nil {
			r.Issues = append(r.Issues, "client wheel: "+err.Error())
		} else {
			r.ClientHits = ch
			if ch[wantRPC] == 0 && ch[wantProto] == 0 {
				r.Issues = append(r.Issues, "client wheel missing magic rpc/protocol markers")
			}
			if ch[oldRPC] > 0 && ch[wantRPC] == 0 {
				r.Issues = append(r.Issues, "client still has frida:rpc without magic:rpc")
			}
			if ch[oldProto] > 0 && ch[wantProto] == 0 {
				r.Issues = append(r.Issues, "client still has re.frida. without re."+magic+".")
			}
			if ch[oldPath] > 0 && ch[wantPath] == 0 {
				r.Issues = append(r.Issues, "client still has /re/frida/ object path without /re/"+magic+"/")
			}
			if ch[wantPath] == 0 && ch[wantProto] > 0 {
				r.Issues = append(r.Issues, "client missing /re/"+magic+"/ object-path markers (causes DBus UNKNOWN_METHOD)")
			}
		}
		r.ClientOK = true
		for _, iss := range r.Issues {
			if strings.HasPrefix(iss, "client") {
				r.ClientOK = false
				break
			}
		}
	} else {
		r.ClientOK = true // not checked
	}

	r.Matched = r.ServerOK && r.ClientOK
	return r, nil
}

func countInZipOrFile(path string, needles []string) (map[string]int, error) {
	out := map[string]int{}
	for _, n := range needles {
		out[n] = 0
	}
	low := strings.ToLower(path)
	if strings.HasSuffix(low, ".whl") || strings.HasSuffix(low, ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return out, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			// stream
			buf := make([]byte, 1<<20)
			var data []byte
			for {
				nr, er := rc.Read(buf)
				if nr > 0 {
					data = append(data, buf[:nr]...)
					// cap single member at 64MB
					if len(data) > 64<<20 {
						break
					}
				}
				if er != nil {
					break
				}
			}
			rc.Close()
			for _, n := range needles {
				out[n] += bytes.Count(data, []byte(n))
			}
		}
		return out, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	for _, n := range needles {
		out[n] = bytes.Count(b, []byte(n))
	}
	return out, nil
}

// FindMagicServerBinary returns first non-empty *server* under catalog entry binaries.
func FindMagicServerBinary(entryDir, magic string) (string, error) {
	bin := filepath.Join(entryDir, "binaries")
	ents, err := os.ReadDir(bin)
	if err != nil {
		return "", err
	}
	var fallback string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !strings.Contains(low, "server") {
			continue
		}
		p := filepath.Join(bin, name)
		st, err := os.Stat(p)
		if err != nil || st.Size() < MinArtifactBytes {
			continue
		}
		if strings.Contains(name, magic) {
			return p, nil
		}
		if fallback == "" {
			fallback = p
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no server binary in %s", bin)
}

// FindHostFridaWheel returns a host frida wheel for the current OS/arch under entry or _host-tools.
func FindHostFridaWheel(catalogRoot, version, magic string) (string, error) {
	// Prefer entry-local then _host-tools
	candidates := []string{
		filepath.Join(catalogRoot, version, "_host-tools", magic, "python", "host"),
	}
	// also scan platform entries
	platRoot := filepath.Join(catalogRoot, version)
	if ents, err := os.ReadDir(platRoot); err == nil {
		for _, e := range ents {
			if e.IsDir() && e.Name() != "_host-tools" {
				candidates = append(candidates, filepath.Join(platRoot, e.Name(), magic, "python", "host"))
			}
		}
	}
	// platform id guess
	ids := []string{"windows-amd64", "windows-arm64", "linux-x86_64", "linux-arm64", "macos-arm64", "macos-x86_64"}
	for _, root := range candidates {
		for _, id := range ids {
			glob := filepath.Join(root, id, "frida-*.whl")
			m, _ := filepath.Glob(glob)
			if len(m) > 0 {
				return m[0], nil
			}
		}
		m, _ := filepath.Glob(filepath.Join(root, "*", "frida-*.whl"))
		if len(m) > 0 {
			return m[0], nil
		}
	}
	return "", fmt.Errorf("no host frida wheel under catalog for %s/%s", version, magic)
}

// RunCatalogProtocolCrossCheck picks the first catalog entry's server binary and a host
// frida wheel (from wheels list or catalog layout) and runs CrossCheckServerClientProtocol.
func RunCatalogProtocolCrossCheck(catalogRoot string, cfg JobConfig, entryDirs, wheels []string) (ProtocolCrossCheckResult, error) {
	var empty ProtocolCrossCheckResult
	if len(entryDirs) == 0 {
		return empty, fmt.Errorf("no catalog entries")
	}
	var serverPath string
	var lastErr error
	for _, e := range entryDirs {
		p, err := FindMagicServerBinary(e, cfg.MagicName)
		if err != nil {
			lastErr = err
			continue
		}
		serverPath = p
		break
	}
	if serverPath == "" {
		if lastErr != nil {
			return empty, lastErr
		}
		return empty, fmt.Errorf("no server binary in catalog entries")
	}
	clientWheel := pickHostFridaWheel(wheels)
	if clientWheel == "" {
		if w, err := FindHostFridaWheel(catalogRoot, cfg.FridaVersion, cfg.MagicName); err == nil {
			clientWheel = w
		}
	}
	return CrossCheckServerClientProtocol(cfg.MagicName, serverPath, clientWheel)
}

// pickHostFridaWheel prefers a native frida-*.whl over pure frida_tools from a path list.
func pickHostFridaWheel(wheels []string) string {
	var toolsFallback string
	for _, w := range wheels {
		base := strings.ToLower(filepath.Base(w))
		if strings.HasPrefix(base, "frida-") && strings.HasSuffix(base, ".whl") {
			return w
		}
		if strings.Contains(base, "frida_tools") || strings.Contains(base, "frida-tools") {
			toolsFallback = w
		}
	}
	return toolsFallback
}

// WriteProtocolCrossCheckReport writes PROTOCOL-SYNC.json (pretty).
func WriteProtocolCrossCheckReport(path string, r ProtocolCrossCheckResult) error {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
