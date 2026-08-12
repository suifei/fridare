package main

import (
  "context"
  "fmt"
  "fridare-gui/internal/rebuild"
  "os"
)

func main() {
  src := os.Args[1]
  cfg := rebuild.JobConfig{
    FridaVersion: "17.16.4",
    MagicName:    "abcde",
    ListenPort:   27142,
    Goals:        "safe hyphen+rpc mod for compile",
  }
  agent := &rebuild.StubAgent{}
  plan, err := agent.PlanMods(context.Background(), cfg, "fridare/mod-safe")
  if err != nil { panic(err) }
  if err := agent.ApplyMods(context.Background(), cfg, plan, src); err != nil { panic(err) }
  fmt.Println("ApplyMods OK")
  data, _ := os.ReadFile(src + "/subprojects/frida-core/lib/agent/agent.vala")
  fmt.Printf("agent.vala start: %.80q\n", string(data))
  if _, err := os.Stat(src + "/subprojects/frida-core/lib/agent/abcde-agent.version"); err != nil {
    fmt.Println("WARN asset rename:", err)
  } else {
    fmt.Println("asset abcde-agent.version OK")
  }
  data2, _ := os.ReadFile(src + "/fridare-mod-plan.json")
  fmt.Println(string(data2))
}