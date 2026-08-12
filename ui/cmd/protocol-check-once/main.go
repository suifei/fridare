package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fridare-gui/internal/rebuild"
)

func main() {
	magic, server, whl := os.Args[1], os.Args[2], os.Args[3]
	tmp := whl + ".patched"
	n, err := rebuild.PatchFridaRPCInWheel(whl, tmp, magic)
	if err != nil {
		fmt.Println("ERR wheel", err)
		os.Exit(1)
	}
	if err := os.Rename(tmp, whl); err != nil {
		// Windows may need remove first
		_ = os.Remove(whl)
		if err2 := os.Rename(tmp, whl); err2 != nil {
			fmt.Println("ERR rename", err2)
			os.Exit(1)
		}
	}
	fmt.Println("wheel replacements", n)
	r, err := rebuild.CrossCheckServerClientProtocol(magic, server, whl)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	fmt.Printf("matched=%v serverOK=%v clientOK=%v\nissues=%v\nserver_hits=%v\nclient_hits=%v\n",
		r.Matched, r.ServerOK, r.ClientOK, r.Issues, r.ServerHits, r.ClientHits)
	out := filepath.Join(filepath.Dir(filepath.Dir(server)), "PROTOCOL-SYNC.json")
	_ = rebuild.WriteProtocolCrossCheckReport(out, r)
	fmt.Println("wrote", out)
	if !r.Matched {
		os.Exit(1)
	}
}
