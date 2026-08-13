# kxmwp 16.7.19 与 17.17.1 构建对照

两批产物都走 **Fridare GUI 源码重编译同一条 JobConfig**（`deep` + 去符号/花指令/行为标记，Docker-only），magic=`kxmwp`。**不是免杀包。**

| | kxmwp-16.7.19 | kxmwp-17.17.1 |
|--|---------------|---------------|
| 官方 clone | **16.7.19**（存在） | **17.17.0**（官方无 17.17.1 tag） |
| 产品 / catalog 版本 | 16.7.19 | 17.17.1（本仓库标签） |
| 工作树 | `fridare-rebuild-16.7.19` | `fridare-rebuild-17.17.1` |
| GUI 路径 | `e2e-rebuild -mode develop` = 源码页 Orchestrator | 同左 |
| linux-x86_64 server | `kxmwp:rpc`=12 · `re.kxmwp.`=48 · `/re/kxmwp/`=21 | `kxmwp:rpc`=13 · `re.kxmwp.`=50 · `/re/kxmwp/`=22 |
| 官方协议残留 | `frida:rpc`=`re.frida.`=`/re/frida/`=0 | 同左 |
| 端口 | `-l`，默认仍 27042 | 同左 |
| 16.x 特例 | 关 `compiler_snapshot`（SDK mksnapshot 不可用） | 17 已无该 meson 选项 |

说明：[16.7.19](./kxmwp-16.7.19.md) · [17.17.1](./kxmwp-17.17.1.md)
