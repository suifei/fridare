package rebuild

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StripResult is the outcome of StripProductBinary.
type StripResult struct {
	BytesIn          int  `json:"bytes_in"`
	BytesOut         int  `json:"bytes_out"`
	ExportsRewritten int  `json:"exports_rewritten"`
	DebugStripped    bool `json:"debug_stripped"`
	Format           string `json:"format"`
}

// ShouldStripProductSymbols reports whether export-time symbol strip is enabled.
// On for deep/abi/full/explore unless DisableSymbolStrip; StripSymbols forces on.
func ShouldStripProductSymbols(cfg JobConfig) bool {
	if cfg.DisableSymbolStrip {
		return false
	}
	if cfg.StripSymbols {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(cfg.DirectionProfile)) {
	case "deep", "abi", "full", "explore":
		return true
	default:
		return false
	}
}

// StripProductBinary strips ELF debug symbol tables and same-length-rewrites
// `frida_` prefixes in dynsym/export name strings. File size is unchanged or smaller.
func StripProductBinary(path, magic string) (StripResult, error) {
	st, err := os.Stat(path)
	if err != nil {
		return StripResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StripResult{}, err
	}
	out, res, err := StripProductBinaryBytes(data, magic)
	if err != nil {
		return res, err
	}
	if bytes.Equal(out, data) {
		return res, nil
	}
	if err := os.WriteFile(path, out, st.Mode()); err != nil {
		return res, err
	}
	return res, nil
}

// StripProductBinaryBytes is the pure transform tests and export both call.
func StripProductBinaryBytes(data []byte, magic string) ([]byte, StripResult, error) {
	res := StripResult{BytesIn: len(data), BytesOut: len(data)}
	if len(data) < 16 {
		return data, res, nil
	}
	out := append([]byte(nil), data...)
	if bytes.HasPrefix(out, []byte("\x7fELF")) {
		res.Format = "elf"
		n, dbg := stripELFInPlace(out, magic)
		// Always also rewrite leftover identifier strings (Vala `_frida_*`,
		// rodata). Dynsym-only pass leaves those in the file.
		n += rewriteFridaExportCStrings(out, magic)
		res.ExportsRewritten = n
		res.DebugStripped = dbg
	} else if len(out) > 64 && out[0] == 'M' && out[1] == 'Z' {
		res.Format = "pe"
		res.ExportsRewritten = rewriteFridaExportCStrings(out, magic)
	} else {
		res.Format = "raw"
		res.ExportsRewritten = rewriteFridaExportCStrings(out, magic)
	}
	res.BytesOut = len(out)
	return out, res, nil
}

// StripProductBinariesDir walks product artifacts and strips each file ≥1KiB.
func StripProductBinariesDir(dir, magic string) (int, error) {
	if dir == "" {
		return 0, fmt.Errorf("dir empty")
	}
	n := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() < MinArtifactBytes || info.Size() > 200*1024*1024 {
			return nil
		}
		res, serr := StripProductBinary(path, magic)
		if serr != nil {
			return nil
		}
		if res.ExportsRewritten > 0 || res.DebugStripped {
			n++
		}
		return nil
	})
	return n, err
}

func rewriteFridaExportCStrings(data []byte, magic string) int {
	if magic == "" || len(magic) != 5 {
		return 0
	}
	n := rewriteIdentPrefix(data, []byte("frida_"), []byte(magic+"_"))
	// Vala emits `_frida_*` and `*_on_frida_thread`; previous byte is `_` so
	// the ident-start pass skips them.
	n += rewriteSameLen(data, []byte("_frida_"), []byte("_"+magic+"_"))
	return n
}

func rewriteIdentPrefix(data, old, neu []byte) int {
	if len(old) != len(neu) || len(old) == 0 {
		return 0
	}
	n := 0
	idx := 0
	for {
		i := bytes.Index(data[idx:], old)
		if i < 0 {
			break
		}
		at := idx + i
		if at > 0 && isIdentCont(data[at-1]) {
			idx = at + 1
			continue
		}
		copy(data[at:at+len(old)], neu)
		n++
		idx = at + len(old)
	}
	return n
}

func rewriteSameLen(data, old, neu []byte) int {
	if len(old) != len(neu) || len(old) == 0 {
		return 0
	}
	n := 0
	idx := 0
	for {
		i := bytes.Index(data[idx:], old)
		if i < 0 {
			break
		}
		at := idx + i
		copy(data[at:at+len(old)], neu)
		n++
		idx = at + len(old)
	}
	return n
}

// stripELFInPlace zeros SHT_SYMTAB and rewrites frida_ names in SHT_DYNSYM strtabs.
func stripELFInPlace(data []byte, magic string) (exports int, debugStripped bool) {
	if len(data) < 64 || data[4] != 2 { // ELFCLASS64
		// still try string rewrite for ELF32
		return rewriteFridaExportCStrings(data, magic), false
	}
	le := data[5] == 1
	if !le {
		return rewriteFridaExportCStrings(data, magic), false
	}
	shoff := binary.LittleEndian.Uint64(data[40:48])
	shentsize := binary.LittleEndian.Uint16(data[58:60])
	shnum := binary.LittleEndian.Uint16(data[60:62])
	if shentsize < 64 || shnum == 0 || shoff == 0 {
		return rewriteFridaExportCStrings(data, magic), false
	}
	type sec struct {
		typ, link uint32
		off, size uint64
		entsize   uint64
	}
	secs := make([]sec, shnum)
	for i := uint16(0); i < shnum; i++ {
		base := int(shoff) + int(i)*int(shentsize)
		if base+64 > len(data) {
			return rewriteFridaExportCStrings(data, magic), false
		}
		secs[i] = sec{
			typ:     binary.LittleEndian.Uint32(data[base+4 : base+8]),
			off:     binary.LittleEndian.Uint64(data[base+24 : base+32]),
			size:    binary.LittleEndian.Uint64(data[base+32 : base+40]),
			link:    binary.LittleEndian.Uint32(data[base+40 : base+44]),
			entsize: binary.LittleEndian.Uint64(data[base+56 : base+64]),
		}
	}
	for _, s := range secs {
		if s.typ == uint32(elf.SHT_SYMTAB) && s.size > 0 && int(s.off+s.size) <= len(data) {
			for i := s.off; i < s.off+s.size; i++ {
				data[i] = 0
			}
			debugStripped = true
		}
		if s.typ == uint32(elf.SHT_DYNSYM) && int(s.link) < len(secs) {
			str := secs[s.link]
			if str.typ == uint32(elf.SHT_STRTAB) && int(str.off+str.size) <= len(data) {
				exports += rewriteFridaNamesInStrtab(data[str.off:str.off+str.size], magic)
			}
		}
	}
	if exports == 0 && !debugStripped {
		exports = rewriteFridaExportCStrings(data, magic)
	}
	return exports, debugStripped
}

func rewriteFridaNamesInStrtab(strtab []byte, magic string) int {
	if magic == "" || len(magic) != 5 {
		return 0
	}
	n := 0
	i := 0
	for i < len(strtab) {
		if strtab[i] == 0 {
			i++
			continue
		}
		end := i
		for end < len(strtab) && strtab[end] != 0 {
			end++
		}
		name := strtab[i:end]
		if bytes.HasPrefix(name, []byte("frida_")) {
			copy(name[:5], []byte(magic))
			n++
		} else if bytes.HasPrefix(name, []byte("_frida_")) {
			copy(name[1:6], []byte(magic))
			n++
		}
		i = end + 1
	}
	return n
}
