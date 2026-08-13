# kxmwp 16.7.19 与 17.17.1 构建对照

两批产物都走 **Fridare GUI 源码重编译同一条 JobConfig**（`deep` + 去符号/花指令/行为标记，Docker-only），magic=`kxmwp`。**不是免杀包。**

| | kxmwp-16.7.19 | kxmwp-17.17.1 |
|--|---------------|---------------|
| 官方 clone | **16.7.19**（存在） | **17.17.0**（官方无 17.17.1 tag） |
| 产品 / catalog 版本 | 16.7.19 | 17.17.1（本仓库标签） |
| 工作树 | `fridare-rebuild-16.7.19` | `fridare-rebuild-17.17.1` |
| 协议 | `kxmwp:rpc` / `re.kxmwp.` / `/re/kxmwp/` | 同左 |
| 端口 | `-l`，默认仍 27042 | 同左 |
| 入口 | GUI 一键深度 或 `e2e-rebuild`（同 Orchestrator） | 同左 |

说明：[16.7.19](./kxmwp-16.7.19.md) · [17.17.1](./kxmwp-17.17.1.md)
