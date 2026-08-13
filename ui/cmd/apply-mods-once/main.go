package main

import (
	"context"
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
)

func main() {
	src := "D:\\works\\fridare-rebuild-17.17.0\\src\\frida"
	magic := "kxmwp"
	if len(os.Args) > 1 {
		src = os.Args[1]
	}
	if len(os.Args) > 2 {
		magic = os.Args[2]
	}
	cfg := rebuild.JobConfig{
		FridaVersion:     "17.17.0",
		MagicName:        magic,
		ListenPort:       27042,
		DirectionProfile: "deep",
		BuildID:          "stealth-r1",
		StripSymbols:     true,
		Goals:            "apply stealth + deep extras on existing tree",
	}
	agent := &rebuild.StubAgent{}
	plan, err := rebuild.PlanModsFromTree(src, cfg, "fridare/stealth-r1")
	if err != nil {
		panic(err)
	}
	fmt.Printf("ops=%d\n", len(plan.Operations))
	if err := agent.ApplyMods(context.Background(), cfg, plan, src); err != nil {
		panic(err)
	}
	fmt.Println("ApplyMods OK")
}
