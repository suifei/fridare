package rebuild

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
)

const dexNoIndex = 0xffffffff

func clipStr(s []byte) string {
	if len(s) > 48 {
		return string(s[:48]) + "…"
	}
	return string(s)
}

// dexStringIdsOrdered reports whether string_ids are in ascending MUTF-8
// byte order (ART rejects the dex otherwise).
func dexStringIdsOrdered(dex []byte) (bool, string) {
	ss, _, err := readDexStringEntries(dex)
	if err != nil {
		return false, err.Error()
	}
	for i := 0; i < len(ss)-1; i++ {
		if bytes.Compare(ss[i], ss[i+1]) > 0 {
			return false, fmt.Sprintf("out-of-order string_ids: %q then %q", clipStr(ss[i]), clipStr(ss[i+1]))
		}
	}
	return true, "ok"
}

func readDexStringEntries(dex []byte) (contents [][]byte, dataOffs []uint32, err error) {
	if len(dex) < 112 || !isDexMagic(dex[:8]) {
		return nil, nil, fmt.Errorf("not a dex")
	}
	n := int(binary.LittleEndian.Uint32(dex[56:60]))
	off := int(binary.LittleEndian.Uint32(dex[60:64]))
	if n < 0 || n > 1_000_000 || off < 0 || off+4*n > len(dex) {
		return nil, nil, fmt.Errorf("bad string_ids")
	}
	contents = make([][]byte, n)
	dataOffs = make([]uint32, n)
	for i := 0; i < n; i++ {
		so := binary.LittleEndian.Uint32(dex[off+4*i:])
		dataOffs[i] = so
		if int(so) >= len(dex) {
			return nil, nil, fmt.Errorf("string_data_off %d", so)
		}
		_, p, e := readULEB128(dex, int(so))
		if e != nil {
			return nil, nil, e
		}
		end := bytes.IndexByte(dex[p:], 0)
		if end < 0 {
			return nil, nil, fmt.Errorf("unterminated string %d", i)
		}
		contents[i] = append([]byte(nil), dex[p:p+end]...)
	}
	return contents, dataOffs, nil
}

// resortDexStringIds reorders string_ids into MUTF-8 ascending order and
// remaps every string index reference. String data stays put; only the id
// table and indices change. Uleb slots keep their original byte width via
// non-canonical encoding when needed.
func resortDexStringIds(dex []byte) (bool, error) {
	contents, dataOffs, err := readDexStringEntries(dex)
	if err != nil {
		return false, err
	}
	n := len(contents)
	if n == 0 {
		return false, nil
	}
	ordered := true
	for i := 0; i < n-1; i++ {
		if bytes.Compare(contents[i], contents[i+1]) > 0 {
			ordered = false
			break
		}
	}
	if ordered {
		return false, nil
	}

	type pair struct {
		old int
		s   []byte
	}
	ps := make([]pair, n)
	for i := 0; i < n; i++ {
		ps[i] = pair{old: i, s: contents[i]}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		return bytes.Compare(ps[i].s, ps[j].s) < 0
	})
	oldToNew := make([]uint32, n)
	strOff := int(binary.LittleEndian.Uint32(dex[60:64]))
	for newIdx, p := range ps {
		oldToNew[p.old] = uint32(newIdx)
		binary.LittleEndian.PutUint32(dex[strOff+4*newIdx:], dataOffs[p.old])
	}
	if err := remapDexStringIndices(dex, oldToNew); err != nil {
		return false, err
	}
	return true, nil
}

func remapDexStringIndices(dex []byte, oldToNew []uint32) error {
	n := len(oldToNew)
	mapU4 := func(off int) {
		v := binary.LittleEndian.Uint32(dex[off:])
		if v == dexNoIndex || int(v) >= n {
			return
		}
		binary.LittleEndian.PutUint32(dex[off:], oldToNew[v])
	}

	typeIdsSize := int(binary.LittleEndian.Uint32(dex[64:68]))
	typeIdsOff := int(binary.LittleEndian.Uint32(dex[68:72]))
	for i := 0; i < typeIdsSize; i++ {
		mapU4(typeIdsOff + 4*i)
	}

	protoSize := int(binary.LittleEndian.Uint32(dex[72:76]))
	protoOff := int(binary.LittleEndian.Uint32(dex[76:80]))
	for i := 0; i < protoSize; i++ {
		mapU4(protoOff + 12*i) // shorty_idx
	}

	fieldSize := int(binary.LittleEndian.Uint32(dex[80:84]))
	fieldOff := int(binary.LittleEndian.Uint32(dex[84:88]))
	for i := 0; i < fieldSize; i++ {
		mapU4(fieldOff + 8*i + 4) // name_idx
	}

	methodSize := int(binary.LittleEndian.Uint32(dex[88:92]))
	methodOff := int(binary.LittleEndian.Uint32(dex[92:96]))
	for i := 0; i < methodSize; i++ {
		mapU4(methodOff + 8*i + 4) // name_idx
	}

	classSize := int(binary.LittleEndian.Uint32(dex[96:100]))
	classOff := int(binary.LittleEndian.Uint32(dex[100:104]))
	seenAnno := map[uint32]bool{}
	seenDebug := map[uint32]bool{}
	seenCode := map[uint32]bool{}
	for i := 0; i < classSize; i++ {
		base := classOff + 32*i
		mapU4(base + 16) // source_file_idx
		annoDir := binary.LittleEndian.Uint32(dex[base+20:])
		if annoDir != 0 && !seenAnno[annoDir] {
			seenAnno[annoDir] = true
			if err := remapAnnotationsDirectory(dex, int(annoDir), oldToNew); err != nil {
				return err
			}
		}
		staticValues := binary.LittleEndian.Uint32(dex[base+28:])
		if staticValues != 0 {
			if _, err := remapEncodedArray(dex, int(staticValues), oldToNew); err != nil {
				return err
			}
		}
		classData := binary.LittleEndian.Uint32(dex[base+24:])
		if classData != 0 {
			if err := remapClassDataCodeStrings(dex, int(classData), oldToNew, seenCode, seenDebug); err != nil {
				return err
			}
		}
	}
	return nil
}

func remapAnnotationsDirectory(dex []byte, off int, oldToNew []uint32) error {
	if off < 0 || off+16 > len(dex) {
		return fmt.Errorf("bad annotations_directory_item")
	}
	classAnno := binary.LittleEndian.Uint32(dex[off:])
	nFields := binary.LittleEndian.Uint32(dex[off+4:])
	nMethods := binary.LittleEndian.Uint32(dex[off+8:])
	nParams := binary.LittleEndian.Uint32(dex[off+12:])
	p := off + 16
	if classAnno != 0 {
		if err := remapAnnotationSet(dex, int(classAnno), oldToNew); err != nil {
			return err
		}
	}
	for i := 0; i < int(nFields); i++ {
		if p+8 > len(dex) {
			return fmt.Errorf("truncated fields annotations")
		}
		aoff := binary.LittleEndian.Uint32(dex[p+4:])
		p += 8
		if aoff != 0 {
			if err := remapAnnotationSet(dex, int(aoff), oldToNew); err != nil {
				return err
			}
		}
	}
	for i := 0; i < int(nMethods); i++ {
		if p+8 > len(dex) {
			return fmt.Errorf("truncated methods annotations")
		}
		aoff := binary.LittleEndian.Uint32(dex[p+4:])
		p += 8
		if aoff != 0 {
			if err := remapAnnotationSet(dex, int(aoff), oldToNew); err != nil {
				return err
			}
		}
	}
	for i := 0; i < int(nParams); i++ {
		if p+8 > len(dex) {
			return fmt.Errorf("truncated params annotations")
		}
		aoff := binary.LittleEndian.Uint32(dex[p+4:])
		p += 8
		if aoff != 0 {
			if err := remapAnnotationSetRefList(dex, int(aoff), oldToNew); err != nil {
				return err
			}
		}
	}
	return nil
}

func remapAnnotationSetRefList(dex []byte, off int, oldToNew []uint32) error {
	if off+4 > len(dex) {
		return fmt.Errorf("bad annotation_set_ref_list")
	}
	size := int(binary.LittleEndian.Uint32(dex[off:]))
	p := off + 4
	for i := 0; i < size; i++ {
		if p+4 > len(dex) {
			return fmt.Errorf("truncated annotation_set_ref_list")
		}
		aoff := binary.LittleEndian.Uint32(dex[p:])
		p += 4
		if aoff != 0 {
			if err := remapAnnotationSet(dex, int(aoff), oldToNew); err != nil {
				return err
			}
		}
	}
	return nil
}

func remapAnnotationSet(dex []byte, off int, oldToNew []uint32) error {
	if off+4 > len(dex) {
		return fmt.Errorf("bad annotation_set_item")
	}
	size := int(binary.LittleEndian.Uint32(dex[off:]))
	p := off + 4
	for i := 0; i < size; i++ {
		if p+4 > len(dex) {
			return fmt.Errorf("truncated annotation_set_item")
		}
		aoff := binary.LittleEndian.Uint32(dex[p:])
		p += 4
		if aoff != 0 {
			if err := remapAnnotationItem(dex, int(aoff), oldToNew); err != nil {
				return err
			}
		}
	}
	return nil
}

func remapAnnotationItem(dex []byte, off int, oldToNew []uint32) error {
	if off >= len(dex) {
		return fmt.Errorf("bad annotation_item")
	}
	_, err := remapEncodedAnnotation(dex, off+1, oldToNew)
	return err
}

func remapEncodedAnnotation(dex []byte, off int, oldToNew []uint32) (int, error) {
	_, p, err := readULEB128(dex, off) // type_idx
	if err != nil {
		return off, err
	}
	size, p, err := readULEB128(dex, p)
	if err != nil {
		return off, err
	}
	for i := 0; i < int(size); i++ {
		p, err = rewriteULEBStringIndex(dex, p, oldToNew)
		if err != nil {
			return off, err
		}
		p, err = remapEncodedValue(dex, p, oldToNew)
		if err != nil {
			return off, err
		}
	}
	return p, nil
}

func remapEncodedArray(dex []byte, off int, oldToNew []uint32) (int, error) {
	size, p, err := readULEB128(dex, off)
	if err != nil {
		return off, err
	}
	for i := 0; i < int(size); i++ {
		p, err = remapEncodedValue(dex, p, oldToNew)
		if err != nil {
			return off, err
		}
	}
	return p, nil
}

func remapEncodedValue(dex []byte, off int, oldToNew []uint32) (int, error) {
	if off >= len(dex) {
		return off, fmt.Errorf("truncated encoded_value")
	}
	hdr := dex[off]
	p := off + 1
	valueType := hdr & 0x1f
	valueArg := int(hdr >> 5)
	switch valueType {
	case 0x00, 0x02, 0x03, 0x04, 0x06, 0x10, 0x11, 0x15, 0x16, 0x18, 0x19, 0x1a, 0x1b:
		// byte/short/char/int/long/float/double + type/field/method/enum/method_* indices
		p += valueArg + 1
	case 0x1c:
		return remapEncodedArray(dex, p, oldToNew)
	case 0x1d:
		return remapEncodedAnnotation(dex, p, oldToNew)
	case 0x1e, 0x1f:
		// null / boolean
	case 0x17: // string — (value_arg+1) little-endian bytes, NOT uleb128
		return rewriteFixedStringIndex(dex, p, valueArg+1, oldToNew)
	default:
		p += valueArg + 1
	}
	return p, nil
}

func rewriteFixedStringIndex(dex []byte, off, width int, oldToNew []uint32) (int, error) {
	if width < 1 || width > 4 || off+width > len(dex) {
		return off, fmt.Errorf("bad fixed string index width %d", width)
	}
	var old uint32
	for i := 0; i < width; i++ {
		old |= uint32(dex[off+i]) << (8 * i)
	}
	if int(old) >= len(oldToNew) {
		return off, fmt.Errorf("string idx %d out of range (fixed)", old)
	}
	neu := oldToNew[old]
	max := uint64(1)<<uint(8*width) - 1
	if width == 4 {
		max = 0xffffffff
	}
	if uint64(neu) > max {
		return off, fmt.Errorf("remapped string idx %d does not fit in %d bytes", neu, width)
	}
	for i := 0; i < width; i++ {
		dex[off+i] = byte(neu >> (8 * i))
	}
	return off + width, nil
}

func rewriteULEBStringIndex(dex []byte, off int, oldToNew []uint32) (int, error) {
	old, end, err := readULEB128(dex, off)
	if err != nil {
		return off, err
	}
	width := end - off
	if int(old) >= len(oldToNew) {
		return end, fmt.Errorf("string idx %d out of range", old)
	}
	if err := writeULEB128Fixed(dex, off, width, oldToNew[old]); err != nil {
		return off, err
	}
	return end, nil
}

func rewriteULEBStringIndexP1(dex []byte, off int, oldToNew []uint32) (int, error) {
	old, end, err := readULEB128(dex, off)
	if err != nil {
		return off, err
	}
	width := end - off
	if old == 0 {
		return end, nil
	}
	idx := old - 1
	if int(idx) >= len(oldToNew) {
		return end, fmt.Errorf("string idx p1 %d out of range", idx)
	}
	neu := uint32(oldToNew[idx]) + 1
	if err := writeULEB128Fixed(dex, off, width, neu); err != nil {
		return off, err
	}
	return end, nil
}

func skipULEB128(dex []byte, off int) (int, error) {
	_, end, err := readULEB128(dex, off)
	return end, err
}

func skipULEB128P1(dex []byte, off int) (int, error) {
	return skipULEB128(dex, off)
}

func readULEB128(data []byte, off int) (uint32, int, error) {
	var result uint32
	var shift uint
	for i := 0; i < 5; i++ {
		if off+i >= len(data) {
			return 0, off, fmt.Errorf("uleb truncated")
		}
		b := data[off+i]
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, off + i + 1, nil
		}
		shift += 7
	}
	return 0, off, fmt.Errorf("uleb too long")
}

func readSLEB128(data []byte, off int) (int32, int, error) {
	var result int32
	var shift uint
	for i := 0; i < 5; i++ {
		if off+i >= len(data) {
			return 0, off, fmt.Errorf("sleb truncated")
		}
		b := data[off+i]
		result |= int32(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			if shift < 32 && b&0x40 != 0 {
				result |= ^((1 << shift) - 1)
			}
			return result, off + i + 1, nil
		}
	}
	return 0, off, fmt.Errorf("sleb too long")
}

func writeULEB128Fixed(data []byte, off, width int, v uint32) error {
	if width <= 0 || off+width > len(data) {
		return fmt.Errorf("bad uleb width")
	}
	max := uint32((uint64(1) << (7 * width)) - 1)
	if width >= 5 {
		max = 0xffffffff
	}
	if v > max {
		return fmt.Errorf("uleb value %d does not fit in %d bytes", v, width)
	}
	for i := 0; i < width; i++ {
		b := byte(v & 0x7f)
		v >>= 7
		if i < width-1 {
			b |= 0x80
		}
		data[off+i] = b
	}
	return nil
}

func remapClassDataCodeStrings(dex []byte, off int, oldToNew []uint32, seenCode, seenDebug map[uint32]bool) error {
	p := off
	var err error
	var staticFields, instanceFields, directMethods, virtualMethods uint32
	staticFields, p, err = readULEB128(dex, p)
	if err != nil {
		return err
	}
	instanceFields, p, err = readULEB128(dex, p)
	if err != nil {
		return err
	}
	directMethods, p, err = readULEB128(dex, p)
	if err != nil {
		return err
	}
	virtualMethods, p, err = readULEB128(dex, p)
	if err != nil {
		return err
	}
	skipFields := func(count uint32) error {
		for i := 0; i < int(count); i++ {
			_, p, err = readULEB128(dex, p)
			if err != nil {
				return err
			}
			_, p, err = readULEB128(dex, p)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err := skipFields(staticFields); err != nil {
		return err
	}
	if err := skipFields(instanceFields); err != nil {
		return err
	}
	walkMethods := func(count uint32) error {
		for i := 0; i < int(count); i++ {
			_, p, err = readULEB128(dex, p)
			if err != nil {
				return err
			}
			_, p, err = readULEB128(dex, p)
			if err != nil {
				return err
			}
			var codeOff uint32
			codeOff, p, err = readULEB128(dex, p)
			if err != nil {
				return err
			}
			if codeOff != 0 && !seenCode[codeOff] {
				seenCode[codeOff] = true
				if err := remapCodeItemStrings(dex, int(codeOff), oldToNew, seenDebug); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walkMethods(directMethods); err != nil {
		return err
	}
	return walkMethods(virtualMethods)
}

func remapCodeItemStrings(dex []byte, off int, oldToNew []uint32, seenDebug map[uint32]bool) error {
	if off+16 > len(dex) {
		return fmt.Errorf("bad code_item")
	}
	debugOff := binary.LittleEndian.Uint32(dex[off+8:])
	insnsSize := binary.LittleEndian.Uint32(dex[off+12:])
	insnsAt := off + 16
	insnsEnd := insnsAt + int(insnsSize)*2
	if insnsEnd > len(dex) {
		return fmt.Errorf("insns truncated")
	}
	if err := remapInsnsStringIndices(dex, insnsAt, insnsEnd, oldToNew); err != nil {
		return err
	}
	if debugOff != 0 && !seenDebug[debugOff] {
		seenDebug[debugOff] = true
		if err := remapDebugInfo(dex, int(debugOff), oldToNew); err != nil {
			return err
		}
	}
	return nil
}

// dexInsnUnits: code units per opcode (payload opcodes handled separately).
var dexInsnUnits = [256]int{
	1, 1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 1, 1, 1, 1, 1, // 0x00-0x0f
	1, 1, 1, 2, 3, 2, 2, 3, 3, 2, 2, 3, 2, 1, 1, 2, // 0x10-0x1f
	2, 1, 2, 2, 3, 3, 3, 1, 1, 2, 3, 3, 3, 2, 2, 2, // 0x20-0x2f
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, // 0x30-0x3f
	1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // 0x40-0x4f
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // 0x50-0x5f
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, // 0x60-0x6f
	3, 3, 3, 1, 3, 3, 3, 3, 3, 1, 1, 1, 1, 1, 1, 1, // 0x70-0x7f
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 0x80-0x8f
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // 0x90-0x9f
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // 0xa0-0xaf
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 0xb0-0xbf
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 0xc0-0xcf
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // 0xd0-0xdf
	2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 0xe0-0xef
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 0xf0-0xff
}

func remapInsnsStringIndices(dex []byte, start, end int, oldToNew []uint32) error {
	n := len(oldToNew)
	skip := map[int]int{} // start → end (byte offsets) of payloads to jump over
	// First pass: record switch/array payloads referenced by relative offsets.
	for i := start; i < end; {
		op := int(dex[i])
		units := dexInsnUnits[op]
		if units < 1 {
			units = 1
		}
		byteLen := units * 2
		if i+byteLen > end {
			break
		}
		if (op == 0x26 || op == 0x2b || op == 0x2c) && i+6 <= end {
			rel := int32(binary.LittleEndian.Uint32(dex[i+2:]))
			payloadAt := i + int(rel)*2
			if payloadAt >= start && payloadAt+4 <= end {
				ident := binary.LittleEndian.Uint16(dex[payloadAt:])
				var plen int
				switch ident {
				case 0x0100: // packed-switch: u2 ident, u2 size, s4 first_key, s4 targets[size]
					size := int(binary.LittleEndian.Uint16(dex[payloadAt+2:]))
					plen = 8 + size*4
				case 0x0200: // sparse-switch: u2 ident, u2 size, s4 keys[size], s4 targets[size]
					size := int(binary.LittleEndian.Uint16(dex[payloadAt+2:]))
					plen = 4 + size*8
				case 0x0300: // fill-array-data: u2 ident, u2 elem_width, u4 size, data
					ew := int(binary.LittleEndian.Uint16(dex[payloadAt+2:]))
					sz := int(binary.LittleEndian.Uint32(dex[payloadAt+4:]))
					raw := 8 + ew*sz
					plen = (raw + 1) &^ 1 // align to 2 bytes
				}
				if plen > 0 && payloadAt+plen <= end {
					skip[payloadAt] = payloadAt + plen
				}
			}
		}
		i += byteLen
	}
	// Second pass: remap const-string*, jumping over recorded payloads.
	i := start
	for i < end {
		if to, ok := skip[i]; ok {
			i = to
			continue
		}
		op := int(dex[i])
		units := dexInsnUnits[op]
		if units < 1 {
			units = 1
		}
		byteLen := units * 2
		if i+byteLen > end {
			return nil
		}
		switch op {
		case 0x1a:
			old := binary.LittleEndian.Uint16(dex[i+2:])
			if int(old) < n {
				binary.LittleEndian.PutUint16(dex[i+2:], uint16(oldToNew[old]))
			}
		case 0x1b:
			old := binary.LittleEndian.Uint32(dex[i+2:])
			if int(old) < n {
				binary.LittleEndian.PutUint32(dex[i+2:], oldToNew[old])
			}
		}
		i += byteLen
	}
	return nil
}

func remapDebugInfo(dex []byte, off int, oldToNew []uint32) error {
	_, p, err := readULEB128(dex, off) // line_start
	if err != nil {
		return err
	}
	paramSize, p, err := readULEB128(dex, p)
	if err != nil {
		return err
	}
	for i := 0; i < int(paramSize); i++ {
		p, err = rewriteULEBStringIndexP1(dex, p, oldToNew)
		if err != nil {
			return err
		}
	}
	for p < len(dex) {
		op := dex[p]
		p++
		switch op {
		case 0x00: // END_SEQUENCE
			return nil
		case 0x01: // ADVANCE_PC
			_, p, err = readULEB128(dex, p)
		case 0x02: // ADVANCE_LINE
			_, p, err = readSLEB128(dex, p)
		case 0x03: // START_LOCAL
			_, p, err = readULEB128(dex, p) // register_num
			if err != nil {
				return err
			}
			p, err = rewriteULEBStringIndexP1(dex, p, oldToNew) // name
			if err != nil {
				return err
			}
			p, err = skipULEB128P1(dex, p) // type_idx p1 — NOT a string
		case 0x04: // START_LOCAL_EXTENDED
			_, p, err = readULEB128(dex, p)
			if err != nil {
				return err
			}
			p, err = rewriteULEBStringIndexP1(dex, p, oldToNew) // name
			if err != nil {
				return err
			}
			p, err = skipULEB128P1(dex, p) // type_idx
			if err != nil {
				return err
			}
			p, err = rewriteULEBStringIndexP1(dex, p, oldToNew) // sig string
		case 0x05: // END_LOCAL
			_, p, err = readULEB128(dex, p)
		case 0x06: // RESTART_LOCAL
			_, p, err = readULEB128(dex, p)
		case 0x07: // SET_PROLOGUE_END
		case 0x08: // SET_EPILOGUE_BEGIN
		case 0x09: // SET_FILE
			p, err = rewriteULEBStringIndexP1(dex, p, oldToNew)
		default:
			// special opcodes 0x0a-0xff: no operands
			if op < 0x0a {
				return fmt.Errorf("unknown debug opcode 0x%02x", op)
			}
		}
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("debug_info missing END")
}
