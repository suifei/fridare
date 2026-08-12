package rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplyMinGWCompatPatches applies small source fixes so Frida 17.x can cross-compile
// to Windows with Ubuntu/Debian mingw-w64 headers (often older than the MSVC SDK).
// Safe no-op when the tree already contains the markers.
func ApplyMinGWCompatPatches(sourceDir string) (int, error) {
	if sourceDir == "" {
		return 0, nil
	}
	n := 0
	// PROCESS_MACHINE_INFORMATION missing in older winnt.h
	targets := []string{
		filepath.Join(sourceDir, "subprojects", "frida-gum", "gum", "backend-windows", "gumprocess-windows.c"),
	}
	// also nested under frida-core's vendored gum if present
	targets = append(targets,
		filepath.Join(sourceDir, "subprojects", "frida-core", "subprojects", "frida-gum", "gum", "backend-windows", "gumprocess-windows.c"),
	)
	const marker = "/* MinGW headers before Windows 11 SDK lack PROCESS_MACHINE_INFORMATION. */"
	const snippet = `
/* MinGW headers before Windows 11 SDK lack PROCESS_MACHINE_INFORMATION. */
#ifndef ProcessMachineTypeInfo
# define ProcessMachineTypeInfo ((PROCESS_INFORMATION_CLASS) 9)
#endif
#ifndef PROCESS_MACHINE_INFORMATION
typedef struct _PROCESS_MACHINE_INFORMATION {
  USHORT ProcessMachine;
  USHORT Res0;
  DWORD MachineAttributes;
} PROCESS_MACHINE_INFORMATION;
#endif
`
	for _, p := range targets {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(b)
		if strings.Contains(s, marker) {
			continue
		}
		// Insert after winternl.h include block
		needle := "#include <winternl.h>\n"
		idx := strings.Index(s, needle)
		if idx < 0 {
			continue
		}
		ins := idx + len(needle)
		out := s[:ins] + snippet + s[ins:]
		if err := os.WriteFile(p, []byte(out), 0644); err != nil {
			return n, err
		}
		n++
	}
	// MinGW: exception_info is a macro in some windows.h revisions
	exceptors := []string{
		filepath.Join(sourceDir, "subprojects", "frida-gum", "gum", "backend-windows", "gumexceptor-windows.c"),
		filepath.Join(sourceDir, "subprojects", "frida-core", "subprojects", "frida-gum", "gum", "backend-windows", "gumexceptor-windows.c"),
	}
	for _, p := range exceptors {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(b)
		if !strings.Contains(s, "EXCEPTION_POINTERS * exception_info") {
			continue
		}
		out := strings.ReplaceAll(s, "EXCEPTION_POINTERS * exception_info", "EXCEPTION_POINTERS * exception_pointers")
		out = strings.ReplaceAll(out, "exception_info->", "exception_pointers->")
		if out == s {
			continue
		}
		if err := os.WriteFile(p, []byte(out), 0644); err != nil {
			return n, err
		}
		n++
	}
	// DNS Service Discovery types (device-monitor-windows.c) for old MinGW
	if k, err := patchFileIfMissing(filepath.Join(sourceDir, "subprojects", "frida-core", "src", "fruity", "device-monitor-windows.c"),
		"DNS_SERVICE_CANCEL browse_handle",
		"/* MinGW (Ubuntu 22.04 era) lacks DNS Service Discovery",
		deviceMonitorWindowsDNSStub); err != nil {
		return n, err
	} else {
		n += k
	}

	// Ensure inject entitlements basename matches magic product rename.
	// RenameMagicAssetFiles should handle this; repair orphaned copies if needed.
	if err := ensureMagicInjectXcent(sourceDir); err != nil {
		return n, err
	}
	if n > 0 {
		_ = os.WriteFile(filepath.Join(sourceDir, "fridare-mingw-compat.txt"),
			[]byte(fmt.Sprintf("mingw_compat_patches=%d\n", n)), 0644)
	}
	return n, nil
}

const deviceMonitorWindowsDNSStub = `
/* MinGW (Ubuntu 22.04 era) lacks DNS Service Discovery types from newer Windows SDKs. */
#ifndef DNS_SERVICE_CANCEL
typedef struct _DNS_SERVICE_CANCEL {
  PVOID reserved;
} DNS_SERVICE_CANCEL;
#endif
#ifndef DNS_REQUEST_PENDING
# define DNS_REQUEST_PENDING 9506L
#endif
#ifndef DNS_QUERY_REQUEST_VERSION2
# define DNS_QUERY_REQUEST_VERSION2 2
#endif
#ifndef DNS_SERVICE_BROWSE_REQUEST
typedef struct _DNS_SERVICE_BROWSE_REQUEST {
  ULONG Version;
  ULONG InterfaceIndex;
  PCWSTR QueryName;
  union {
    PDNS_SERVICE_BROWSE_CALLBACK pBrowseCallback;
    DNS_QUERY_COMPLETION_ROUTINE pBrowseCallbackV2;
  };
  PVOID pQueryContext;
} DNS_SERVICE_BROWSE_REQUEST;
#endif
#ifndef DNS_QUERY_RESULT
typedef struct _DNS_QUERY_RESULT {
  ULONG Version;
  DNS_STATUS QueryStatus;
  ULONG64 QueryOptions;
  PDNS_RECORD pQueryRecords;
  PVOID Reserved;
} DNS_QUERY_RESULT, *PDNS_QUERY_RESULT;
#endif
`

// patchFileIfMissing inserts stub after "#include <windns.h>" when haystack present and marker absent.
func patchFileIfMissing(path, haystack, marker, stub string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil
	}
	s := string(b)
	if !strings.Contains(s, haystack) || strings.Contains(s, marker) {
		return 0, nil
	}
	needle := "#include <windns.h>\n"
	idx := strings.Index(s, needle)
	if idx < 0 {
		return 0, nil
	}
	ins := idx + len(needle)
	out := s[:ins] + stub + s[ins:]
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		return 0, err
	}
	return 1, nil
}

func ensureMagicInjectXcent(sourceDir string) error {
	// Walk for *inject*.xcent under frida-core
	core := filepath.Join(sourceDir, "subprojects", "frida-core", "inject")
	ents, err := os.ReadDir(core)
	if err != nil {
		return nil
	}
	var hasFrida, hasMagic string
	for _, e := range ents {
		name := e.Name()
		if !strings.HasSuffix(name, ".xcent") {
			continue
		}
		if strings.HasPrefix(name, "frida-") {
			hasFrida = filepath.Join(core, name)
		}
		if !strings.HasPrefix(name, "frida-") {
			hasMagic = filepath.Join(core, name)
		}
	}
	// if meson references abcde-inject.xcent but only frida-inject.xcent exists, copy
	meson := filepath.Join(core, "meson.build")
	mb, err := os.ReadFile(meson)
	if err != nil {
		return nil
	}
	ms := string(mb)
	if strings.Contains(ms, "frida-inject.xcent") {
		return nil // still stock name
	}
	// find magic-inject.xcent reference
	if hasFrida != "" && hasMagic == "" {
		// derive magic name from meson
		// e.g. input: [raw_inject, 'abcde-inject.xcent']
		for _, part := range strings.Split(ms, "'") {
			if strings.HasSuffix(part, "-inject.xcent") && !strings.HasPrefix(part, "frida-") {
				dst := filepath.Join(core, part)
				data, err := os.ReadFile(hasFrida)
				if err != nil {
					return err
				}
				return os.WriteFile(dst, data, 0644)
			}
		}
	}
	return nil
}
