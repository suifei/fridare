package rebuild

import (
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeELF64DynsymFixture writes a tiny ELF64 LE with a dynsym export named exportName.
func writeELF64DynsymFixture(t *testing.T, path, exportName string) []byte {
	t.Helper()
	// Layout:
	//  0: Ehdr 64
	// 64: Phdr 56
	// 120: .text 16
	// 136: .dynsym 48 (2 * 24)
	// 184: .dynstr
	// 216: .shstrtab
	// 256: Shdr[5]*64 = 320 → end 576
	const (
		ehsize    = 64
		phoff     = 64
		phentsize = 56
		textoff   = 120
		textsz    = 16
		symoff    = 136
		syment    = 24
		nsym      = 2
		stroff    = 184
	)
	dynstr := append([]byte{0}, append([]byte(exportName), 0)...)
	for len(dynstr) < 32 {
		dynstr = append(dynstr, 0)
	}
	shstr := []byte("\x00.text\x00.dynsym\x00.dynstr\x00.shstrtab\x00")
	for len(shstr) < 40 {
		shstr = append(shstr, 0)
	}
	shstrOff := 216
	shoff := 256
	shnum := 5
	buf := make([]byte, shoff+shnum*64)

	// e_ident
	copy(buf[0:], []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	binary.LittleEndian.PutUint16(buf[16:], 3)  // ET_DYN
	binary.LittleEndian.PutUint16(buf[18:], 62) // EM_X86_64
	binary.LittleEndian.PutUint32(buf[20:], 1)
	binary.LittleEndian.PutUint64(buf[32:], phoff)
	binary.LittleEndian.PutUint64(buf[40:], uint64(shoff))
	binary.LittleEndian.PutUint16(buf[52:], ehsize)
	binary.LittleEndian.PutUint16(buf[54:], phentsize)
	binary.LittleEndian.PutUint16(buf[56:], 1)
	binary.LittleEndian.PutUint16(buf[58:], 64)
	binary.LittleEndian.PutUint16(buf[60:], uint16(shnum))
	binary.LittleEndian.PutUint16(buf[62:], 4) // shstrndx

	// PT_LOAD dummy
	binary.LittleEndian.PutUint32(buf[phoff:], 1)
	binary.LittleEndian.PutUint32(buf[phoff+4:], 5)
	binary.LittleEndian.PutUint64(buf[phoff+32:], uint64(len(buf)))

	for i := 0; i < textsz; i++ {
		buf[textoff+i] = 0x90
	}

	// dynsym[1].st_name = 1
	st1 := symoff + syment
	binary.LittleEndian.PutUint32(buf[st1:], 1)
	buf[st1+4] = 0x12 // GLOBAL FUNC
	copy(buf[stroff:], dynstr)
	copy(buf[shstrOff:], shstr)

	writeShdr := func(i int, name, typ uint32, off, size, link, entsize uint64) {
		b := shoff + i*64
		binary.LittleEndian.PutUint32(buf[b:], name)
		binary.LittleEndian.PutUint32(buf[b+4:], typ)
		binary.LittleEndian.PutUint64(buf[b+24:], off)
		binary.LittleEndian.PutUint64(buf[b+32:], size)
		binary.LittleEndian.PutUint32(buf[b+40:], uint32(link))
		binary.LittleEndian.PutUint64(buf[b+56:], entsize)
	}
	// 0 NULL
	writeShdr(1, 1, uint32(elf.SHT_PROGBITS), textoff, textsz, 0, 0)                  // .text
	writeShdr(2, 7, uint32(elf.SHT_DYNSYM), symoff, syment*nsym, 3, syment)           // .dynsym → .dynstr
	writeShdr(3, 15, uint32(elf.SHT_STRTAB), stroff, uint64(len(dynstr)), 0, 0)        // .dynstr
	writeShdr(4, 23, uint32(elf.SHT_STRTAB), uint64(shstrOff), uint64(len(shstr)), 0, 0)

	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestStripProductBinary_RewritesDynsymExport(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lib.so")
	orig := writeELF64DynsymFixture(t, p, "frida_agent")
	if f0, err := elf.Open(p); err != nil {
		t.Fatalf("fixture not a valid ELF: %v", err)
	} else {
		_ = f0.Close()
	}
	res, err := StripProductBinary(p, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExportsRewritten < 1 {
		t.Fatalf("expected export rewrite, got %+v", res)
	}
	st, _ := os.Stat(p)
	if int(st.Size()) > len(orig) {
		t.Fatalf("file grew: %d > %d", st.Size(), len(orig))
	}
	out, _ := os.ReadFile(p)
	if string(out) == string(orig) {
		t.Fatal("strip was a no-op")
	}
	f, err := elf.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	syms, err := f.DynamicSymbols()
	if err != nil {
		t.Fatal(err)
	}
	foundNew, foundOld := false, false
	for _, s := range syms {
		if s.Name == "frida_agent" {
			foundOld = true
		}
		if s.Name == "abcde_agent" {
			foundNew = true
		}
	}
	if foundOld {
		t.Fatalf("old export remains: %+v", syms)
	}
	if !foundNew {
		t.Fatalf("new export missing: %+v", syms)
	}
}

func TestRewriteFridaExportCStrings_LeadingUnderscoreAndInterior(t *testing.T) {
	data := []byte("X_frida_device_do_close\x00on_frida_thread\x00frida_agent\x00keep")
	n := rewriteFridaExportCStrings(data, "abcde")
	if n < 2 {
		t.Fatalf("rewrites=%d data=%q", n, data)
	}
	s := string(data)
	if strings.Contains(s, "_frida_") || strings.Contains(s, "\x00frida_agent") {
		t.Fatalf("old prefix remains: %q", data)
	}
	if !strings.Contains(s, "_abcde_device_do_close") || !strings.Contains(s, "on_abcde_thread") || !strings.Contains(s, "abcde_agent") {
		t.Fatalf("new prefix missing: %q", data)
	}
}

func TestFormatStealthJobSummary(t *testing.T) {
	s := FormatStealthJobSummary(JobConfig{DirectionProfile: "deep"})
	if !strings.Contains(s, "strip=true") || !strings.Contains(s, "junk=true") ||
		!strings.Contains(s, "markers=true") || !strings.Contains(s, "不是免杀") {
		t.Fatalf("%s", s)
	}
	s2 := FormatStealthJobSummary(JobConfig{DirectionProfile: "deep", DisableJunk: true, DisableStealthMarkers: true, DisableSymbolStrip: true})
	if !strings.Contains(s2, "strip=false") || !strings.Contains(s2, "junk=false") || !strings.Contains(s2, "markers=false") {
		t.Fatalf("%s", s2)
	}
}

func TestShouldStripProductSymbols(t *testing.T) {
	if !ShouldStripProductSymbols(JobConfig{DirectionProfile: "deep"}) {
		t.Fatal("deep defaults to strip")
	}
	if ShouldStripProductSymbols(JobConfig{DirectionProfile: "safe"}) {
		t.Fatal("safe default off")
	}
	if !ShouldStripProductSymbols(JobConfig{DirectionProfile: "safe", StripSymbols: true}) {
		t.Fatal("force on")
	}
	if ShouldStripProductSymbols(JobConfig{DirectionProfile: "deep", DisableSymbolStrip: true}) {
		t.Fatal("disable")
	}
}
