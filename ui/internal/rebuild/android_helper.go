package rebuild

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"hash/adler32"
	"strings"
)

// PatchAndroidHelperInBinary fixes two post-rename traps that abort stock
// Android frida-server after magic rewrite:
//
//  1. Embedded helper dex still has Java descriptors Lre/frida/HelperBackend
//     while native looks up re.{magic}.HelperBackend. ART then fails to load
//     the class → linux-host-session.vala:744 backend_class != null.
//     Pair `/re/frida/` does not match `Lre/frida/` (no leading slash).
//  2. Same-length rename of dynsym frida_agent_main → {magic}_agent_main
//     leaves DT_GNU_HASH / bloom on the old name → undefined symbol.
//
// Size is unchanged. Safe to run after PatchArtifactBinaryMarkers string replace.
func PatchAndroidHelperInBinary(data []byte, magic string) int {
	if len(magic) != 5 || len(data) < 64 {
		return 0
	}
	n := 0
	n += patchEmbeddedDexJavaPackage(data, magic)
	n += resyncExportHashesAllELFs(data, magic)
	return n
}

func patchEmbeddedDexJavaPackage(data []byte, magic string) int {
	old := []byte("re/frida")
	neu := []byte("re/" + magic)
	if len(old) != len(neu) {
		return 0
	}
	n := 0
	off := 0
	for {
		i := bytes.Index(data[off:], []byte("dex\n"))
		if i < 0 {
			break
		}
		at := off + i
		if at+40 > len(data) {
			break
		}
		if !isDexMagic(data[at : at+8]) {
			off = at + 4
			continue
		}
		size := int(binary.LittleEndian.Uint32(data[at+32 : at+36]))
		if size < 112 || size > 2*1024*1024 || at+size > len(data) {
			off = at + 4
			continue
		}
		dex := data[at : at+size]
		c := bytes.Count(dex, old)
		if c > 0 {
			copy(dex, bytes.ReplaceAll(dex, old, neu))
			fixDexChecksum(dex)
			n += c
		} else if bytes.Contains(dex, neu) {
			// already renamed (whole-file replace) but checksums stale
			before := append([]byte(nil), dex[:32]...)
			fixDexChecksum(dex)
			if !bytes.Equal(before, dex[:32]) {
				n++
			}
		}
		off = at + size
	}
	return n
}

func isDexMagic(m []byte) bool {
	if len(m) < 8 || !bytes.HasPrefix(m, []byte("dex\n")) || m[7] != 0 {
		return false
	}
	// dex\n035\0 / 037\0 / 038\0 / 039\0
	return m[4] >= '0' && m[4] <= '9' && m[5] >= '0' && m[5] <= '9' && m[6] >= '0' && m[6] <= '9'
}

func fixDexChecksum(dex []byte) {
	if len(dex) < 32 {
		return
	}
	sum := sha1.Sum(dex[32:])
	copy(dex[12:32], sum[:])
	binary.LittleEndian.PutUint32(dex[8:12], adler32.Checksum(dex[12:]))
}

func resyncExportHashesAllELFs(data []byte, magic string) int {
	n := 0
	off := 0
	for {
		i := bytes.Index(data[off:], []byte{0x7f, 'E', 'L', 'F'})
		if i < 0 {
			break
		}
		at := off + i
		size := elfImageSize(data[at:])
		if size >= 256 && at+size <= len(data) {
			img := data[at : at+size]
			// Only touch ELFs that actually export *agent_main. Blindly
			// rewriting every nested 0x7fELF (and the outer server image)
			// smashes GumJS / internal-agent.js — 16.x then dies with
			// "Kxmwp is not defined" while enumerating processes.
			if elfExportsAgentMain(img) {
				n += resyncGnuHash(img)
				n += resyncSysvHash(img)
				// GumJS registers globalThis.Frida. Deep hexreplace already
				// turned Frida.* uses into {Magic}.* so 16.x internal-agent.js
				// throws "Kxmwp is not defined". Do NOT write Frida. back
				// (detection string). Rename leftover JS identifier "Frida"
				// in the agent ELF to PascalCase(magic). Only whole tokens —
				// not Friday, FridaScriptOptions, rawFridaType.
				n += renameAgentGumJSGlobal(img, magic)
			}
		}
		// Nested ELFs (embedded agent/helper) sit inside the outer image;
		// do not skip forward by outer size.
		off = at + 4
	}
	return n
}

func elfImageSize(data []byte) int {
	if len(data) < 64 || data[0] != 0x7f {
		return 0
	}
	cls := data[4]
	le := data[5] == 1
	if !le {
		return 0
	}
	var phoff uint64
	var phentsize, phnum uint16
	var shoff uint64
	var shentsize, shnum uint16
	switch cls {
	case 1: // ELF32
		if len(data) < 52 {
			return 0
		}
		phoff = uint64(binary.LittleEndian.Uint32(data[28:32]))
		phentsize = binary.LittleEndian.Uint16(data[42:44])
		phnum = binary.LittleEndian.Uint16(data[44:46])
		shoff = uint64(binary.LittleEndian.Uint32(data[32:36]))
		shentsize = binary.LittleEndian.Uint16(data[46:48])
		shnum = binary.LittleEndian.Uint16(data[48:50])
	case 2: // ELF64
		phoff = binary.LittleEndian.Uint64(data[32:40])
		phentsize = binary.LittleEndian.Uint16(data[54:56])
		phnum = binary.LittleEndian.Uint16(data[56:58])
		shoff = binary.LittleEndian.Uint64(data[40:48])
		shentsize = binary.LittleEndian.Uint16(data[58:60])
		shnum = binary.LittleEndian.Uint16(data[60:62])
	default:
		return 0
	}
	if phnum > 128 || shnum > 512 || phentsize > 128 {
		return 0
	}
	end := 0
	for i := uint16(0); i < phnum; i++ {
		p := int(phoff) + int(i)*int(phentsize)
		if p < 0 || p+int(phentsize) > len(data) {
			return 0
		}
		var poff, psz uint64
		if cls == 1 {
			poff = uint64(binary.LittleEndian.Uint32(data[p+4 : p+8]))
			psz = uint64(binary.LittleEndian.Uint32(data[p+16 : p+20]))
		} else {
			poff = binary.LittleEndian.Uint64(data[p+8 : p+16])
			psz = binary.LittleEndian.Uint64(data[p+32 : p+40])
		}
		if e := int(poff + psz); e > end {
			end = e
		}
	}
	if shoff > 0 && shnum > 0 && shentsize >= 40 {
		if e := int(shoff) + int(shnum)*int(shentsize); e > end && e <= len(data) {
			end = e
		}
	}
	if end < 64 || end > len(data) {
		return 0
	}
	return end
}

func resyncGnuHash(img []byte) int {
	cls := img[4]
	var shoff uint64
	var shentsize, shnum, shstrndx uint16
	switch cls {
	case 1:
		if len(img) < 52 {
			return 0
		}
		shoff = uint64(binary.LittleEndian.Uint32(img[32:36]))
		shentsize = binary.LittleEndian.Uint16(img[46:48])
		shnum = binary.LittleEndian.Uint16(img[48:50])
		shstrndx = binary.LittleEndian.Uint16(img[50:52])
	case 2:
		if len(img) < 64 {
			return 0
		}
		shoff = binary.LittleEndian.Uint64(img[40:48])
		shentsize = binary.LittleEndian.Uint16(img[58:60])
		shnum = binary.LittleEndian.Uint16(img[60:62])
		shstrndx = binary.LittleEndian.Uint16(img[62:64])
	default:
		return 0
	}
	if shnum == 0 || shentsize < 40 || shoff == 0 {
		return 0
	}
	type sec struct {
		typ, link uint32
		off, size, entsize, addr uint64
		name                     uint32
	}
	secs := make([]sec, shnum)
	for i := uint16(0); i < shnum; i++ {
		b := int(shoff) + int(i)*int(shentsize)
		if b+int(shentsize) > len(img) {
			return 0
		}
		if cls == 1 {
			secs[i] = sec{
				name:    binary.LittleEndian.Uint32(img[b : b+4]),
				typ:     binary.LittleEndian.Uint32(img[b+4 : b+8]),
				addr:    uint64(binary.LittleEndian.Uint32(img[b+12 : b+16])),
				off:     uint64(binary.LittleEndian.Uint32(img[b+16 : b+20])),
				size:    uint64(binary.LittleEndian.Uint32(img[b+20 : b+24])),
				link:    binary.LittleEndian.Uint32(img[b+24 : b+28]),
				entsize: uint64(binary.LittleEndian.Uint32(img[b+36 : b+40])),
			}
		} else {
			secs[i] = sec{
				name:    binary.LittleEndian.Uint32(img[b : b+4]),
				typ:     binary.LittleEndian.Uint32(img[b+4 : b+8]),
				addr:    binary.LittleEndian.Uint64(img[b+16 : b+24]),
				off:     binary.LittleEndian.Uint64(img[b+24 : b+32]),
				size:    binary.LittleEndian.Uint64(img[b+32 : b+40]),
				link:    binary.LittleEndian.Uint32(img[b+40 : b+44]),
				entsize: binary.LittleEndian.Uint64(img[b+56 : b+64]),
			}
		}
	}
	if int(shstrndx) >= len(secs) {
		return 0
	}
	str := secs[shstrndx]
	if int(str.off+str.size) > len(img) {
		return 0
	}
	shstr := img[str.off : str.off+str.size]
	secName := func(off uint32) string {
		if int(off) >= len(shstr) {
			return ""
		}
		z := bytes.IndexByte(shstr[off:], 0)
		if z < 0 {
			return string(shstr[off:])
		}
		return string(shstr[off : int(off)+z])
	}
	var gnu, dynsym, dynstr *sec
	for i := range secs {
		switch secName(secs[i].name) {
		case ".gnu.hash":
			gnu = &secs[i]
		case ".dynsym":
			dynsym = &secs[i]
		case ".dynstr":
			dynstr = &secs[i]
		}
	}
	if gnu == nil || dynsym == nil || dynstr == nil {
		return 0
	}
	if int(gnu.off+gnu.size) > len(img) || int(dynsym.off+dynsym.size) > len(img) || int(dynstr.off+dynstr.size) > len(img) {
		return 0
	}
	nbuckets := binary.LittleEndian.Uint32(img[gnu.off : gnu.off+4])
	symoffset := binary.LittleEndian.Uint32(img[gnu.off+4 : gnu.off+8])
	bloomSize := binary.LittleEndian.Uint32(img[gnu.off+8 : gnu.off+12])
	bloomShift := binary.LittleEndian.Uint32(img[gnu.off+12 : gnu.off+16])
	if nbuckets == 0 || bloomSize == 0 || nbuckets > 1<<20 || bloomSize > 1<<20 {
		return 0
	}
	wordBits := uint32(32)
	wordBytes := 4
	symEnt := uint64(16)
	if cls == 2 {
		wordBits = 64
		wordBytes = 8
		symEnt = 24
	}
	if dynsym.entsize != 0 {
		symEnt = dynsym.entsize
	}
	bloomOff := gnu.off + 16
	bucketOff := bloomOff + uint64(bloomSize)*uint64(wordBytes)
	chainOff := bucketOff + uint64(nbuckets)*4
	nsym := int(dynsym.size / symEnt)
	ds := img[dynstr.off : dynstr.off+dynstr.size]
	n := 0
	for i := int(symoffset); i < nsym; i++ {
		so := int(dynsym.off) + i*int(symEnt)
		if so+4 > len(img) {
			break
		}
		stName := binary.LittleEndian.Uint32(img[so : so+4])
		if int(stName) >= len(ds) {
			continue
		}
		z := bytes.IndexByte(ds[stName:], 0)
		if z <= 0 {
			continue
		}
		name := ds[stName : int(stName)+z]
		h := gnuHash(name)
		ci := i - int(symoffset)
		chp := int(chainOff) + ci*4
		if chp+4 > len(img) {
			break
		}
		old := binary.LittleEndian.Uint32(img[chp : chp+4])
		neu := (h &^ 1) | (old & 1)
		if neu != old {
			binary.LittleEndian.PutUint32(img[chp:chp+4], neu)
			n++
		}
		word := (h / wordBits) % bloomSize
		bit1 := h % wordBits
		bit2 := (h >> bloomShift) % wordBits
		bp := int(bloomOff) + int(word)*wordBytes
		if cls == 2 {
			if bp+8 > len(img) {
				continue
			}
			v := binary.LittleEndian.Uint64(img[bp : bp+8])
			mask := (uint64(1) << bit1) | (uint64(1) << bit2)
			if v&mask != mask {
				binary.LittleEndian.PutUint64(img[bp:bp+8], v|mask)
				n++
			}
		} else {
			if bp+4 > len(img) {
				continue
			}
			v := binary.LittleEndian.Uint32(img[bp : bp+4])
			mask := (uint32(1) << bit1) | (uint32(1) << bit2)
			if v&mask != mask {
				binary.LittleEndian.PutUint32(img[bp:bp+4], v|mask)
				n++
			}
		}
	}
	return n
}

func renameAgentGumJSGlobal(img []byte, magic string) int {
	if len(magic) != 5 {
		return 0
	}
	pas := strings.ToUpper(magic[:1]) + magic[1:]
	old := []byte("Frida")
	neu := []byte(pas)
	if len(old) != len(neu) || bytes.Equal(old, neu) {
		return 0
	}
	n := 0
	off := 0
	for {
		i := bytes.Index(img[off:], old)
		if i < 0 {
			break
		}
		at := off + i
		prevOK := at == 0 || !isIdentByte(img[at-1])
		next := at + len(old)
		nextOK := next >= len(img) || !isIdentByte(img[next])
		if prevOK && nextOK {
			copy(img[at:next], neu)
			n++
		}
		off = at + len(old)
	}
	return n
}

func isIdentByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

func elfExportsAgentMain(img []byte) bool {
	secs, ok := parseELFNamedSections(img)
	if !ok {
		return false
	}
	dynstr := secs[".dynstr"]
	if dynstr.size == 0 || int(dynstr.off+dynstr.size) > len(img) {
		return false
	}
	ds := img[dynstr.off : dynstr.off+dynstr.size]
	return bytes.Contains(ds, []byte("frida_agent_main\x00")) || bytes.Contains(ds, []byte("_agent_main\x00"))
}

func gnuHash(name []byte) uint32 {
	h := uint32(5381)
	for _, c := range name {
		h = h*33 + uint32(c)
	}
	return h
}

func sysvHash(name []byte) uint32 {
	var h uint32
	for _, c := range name {
		h = (h << 4) + uint32(c)
		if g := h & 0xf0000000; g != 0 {
			h ^= g >> 24
			h &^= g
		}
	}
	return h
}

func resyncSysvHash(img []byte) int {
	cls := img[4]
	secs, ok := parseELFNamedSections(img)
	if !ok {
		return 0
	}
	hash := secs[".hash"]
	dynsym := secs[".dynsym"]
	dynstr := secs[".dynstr"]
	if hash.off == 0 || dynsym.off == 0 || dynstr.off == 0 {
		return 0
	}
	if int(hash.off+hash.size) > len(img) || int(dynsym.off+dynsym.size) > len(img) || int(dynstr.off+dynstr.size) > len(img) {
		return 0
	}
	if hash.size < 8 {
		return 0
	}
	nbucket := binary.LittleEndian.Uint32(img[hash.off : hash.off+4])
	nchain := binary.LittleEndian.Uint32(img[hash.off+4 : hash.off+8])
	if nbucket == 0 || nchain == 0 || nbucket > 1<<20 || nchain > 1<<20 {
		return 0
	}
	need := 8 + (nbucket+nchain)*4
	if uint64(need) > hash.size {
		return 0
	}
	symEnt := uint64(16)
	if cls == 2 {
		symEnt = 24
	}
	if dynsym.entsize != 0 {
		symEnt = dynsym.entsize
	}
	nsym := int(dynsym.size / symEnt)
	if nsym > int(nchain) {
		nsym = int(nchain)
	}
	ds := img[dynstr.off : dynstr.off+dynstr.size]
	bucketOff := hash.off + 8
	chainOff := bucketOff + uint64(nbucket)*4
	// rebuild: bucket[h] = i; chain[i] = previous bucket[h]
	for i := uint32(0); i < nbucket; i++ {
		binary.LittleEndian.PutUint32(img[int(bucketOff)+int(i)*4:], 0)
	}
	for i := uint32(0); i < nchain; i++ {
		binary.LittleEndian.PutUint32(img[int(chainOff)+int(i)*4:], 0)
	}
	n := 0
	for i := nsym - 1; i >= 1; i-- {
		so := int(dynsym.off) + i*int(symEnt)
		stName := binary.LittleEndian.Uint32(img[so : so+4])
		if int(stName) >= len(ds) {
			continue
		}
		z := bytes.IndexByte(ds[stName:], 0)
		if z <= 0 {
			continue
		}
		h := sysvHash(ds[stName:int(stName)+z]) % nbucket
		prev := binary.LittleEndian.Uint32(img[int(bucketOff)+int(h)*4:])
		binary.LittleEndian.PutUint32(img[int(chainOff)+i*4:], prev)
		binary.LittleEndian.PutUint32(img[int(bucketOff)+int(h)*4:], uint32(i))
		n++
	}
	return n
}

type namedSec struct {
	typ, link            uint32
	off, size, entsize, addr uint64
	name                 uint32
}

func parseELFNamedSections(img []byte) (map[string]namedSec, bool) {
	out := map[string]namedSec{}
	if len(img) < 52 {
		return out, false
	}
	cls := img[4]
	var shoff uint64
	var shentsize, shnum, shstrndx uint16
	switch cls {
	case 1:
		shoff = uint64(binary.LittleEndian.Uint32(img[32:36]))
		shentsize = binary.LittleEndian.Uint16(img[46:48])
		shnum = binary.LittleEndian.Uint16(img[48:50])
		shstrndx = binary.LittleEndian.Uint16(img[50:52])
	case 2:
		if len(img) < 64 {
			return out, false
		}
		shoff = binary.LittleEndian.Uint64(img[40:48])
		shentsize = binary.LittleEndian.Uint16(img[58:60])
		shnum = binary.LittleEndian.Uint16(img[60:62])
		shstrndx = binary.LittleEndian.Uint16(img[62:64])
	default:
		return out, false
	}
	if shnum == 0 || shentsize < 40 || shoff == 0 || shnum > 512 {
		return out, false
	}
	secs := make([]namedSec, shnum)
	for i := uint16(0); i < shnum; i++ {
		b := int(shoff) + int(i)*int(shentsize)
		if b+int(shentsize) > len(img) {
			return out, false
		}
		if cls == 1 {
			secs[i] = namedSec{
				name:    binary.LittleEndian.Uint32(img[b : b+4]),
				typ:     binary.LittleEndian.Uint32(img[b+4 : b+8]),
				addr:    uint64(binary.LittleEndian.Uint32(img[b+12 : b+16])),
				off:     uint64(binary.LittleEndian.Uint32(img[b+16 : b+20])),
				size:    uint64(binary.LittleEndian.Uint32(img[b+20 : b+24])),
				link:    binary.LittleEndian.Uint32(img[b+24 : b+28]),
				entsize: uint64(binary.LittleEndian.Uint32(img[b+36 : b+40])),
			}
		} else {
			secs[i] = namedSec{
				name:    binary.LittleEndian.Uint32(img[b : b+4]),
				typ:     binary.LittleEndian.Uint32(img[b+4 : b+8]),
				addr:    binary.LittleEndian.Uint64(img[b+16 : b+24]),
				off:     binary.LittleEndian.Uint64(img[b+24 : b+32]),
				size:    binary.LittleEndian.Uint64(img[b+32 : b+40]),
				link:    binary.LittleEndian.Uint32(img[b+40 : b+44]),
				entsize: binary.LittleEndian.Uint64(img[b+56 : b+64]),
			}
		}
	}
	if int(shstrndx) >= len(secs) {
		return out, false
	}
	str := secs[shstrndx]
	if int(str.off+str.size) > len(img) {
		return out, false
	}
	shstr := img[str.off : str.off+str.size]
	for i := range secs {
		off := secs[i].name
		if int(off) >= len(shstr) {
			continue
		}
		z := bytes.IndexByte(shstr[off:], 0)
		var name string
		if z < 0 {
			name = string(shstr[off:])
		} else {
			name = string(shstr[off : int(off)+z])
		}
		if name != "" {
			out[name] = secs[i]
		}
	}
	return out, true
}
