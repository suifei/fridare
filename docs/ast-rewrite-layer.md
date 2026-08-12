# 源码魔改的机械安全层：按语言 AST / 词法改写

Frida 源码是**多语言**仓库（C/C++、Vala、Python、Meson、JS…）。  
**不存在「一个 AST 解析整棵 monorepo」的安全模型。** 正确做法是：

```text
策略层（改什么 / 禁什么：L1–L12、profile=safe|deep）
    ↓
分语言引擎（机械安全）
  - C/C++/ObjC/.h：词法分类（c-lex）— 只改字符串与注释
  - Python：优先 CPython **tokenize** 只改 STRING（保留 shebang/注释；**不用** `ast.unparse`）；否则纯 Go 词法（python-lex）
  - Vala：无稳定公共 rewrite AST → 暂用 C-like 词法（仅字符串/注释）
  - Meson/其它文本：generic 词法（引号串）
    ↓
编译门禁 / 产物二进制扫尾（L5）
```

## 当前覆盖

| 语言 | 引擎 | 安全保证 | 未覆盖 |
|------|------|----------|--------|
| C/C++/h | `RewriteCSource` / `c-lex` | 未引号标识符不改；仅产品/RPC 等**具体**标记可改 | 宏拼接边缘情况 |
| Python | **`python-tokenize`（优先）** 或 `python-lex` | 只改 STRING token，保留 shebang/注释/布局；改后 `ast.parse` 校验 | f-string 表达式内标识符 |
| Vala | c-lex + **idents** | 字符串规则 + `namespace Frida` / `frida_*` / `Frida*` token 重命名 | 不做磁盘目录 `frida-core` 改名 |
| Meson | generic-lex + idents | 引号内 + 标识符；**可能**改 dependency 符号名（需完整编译验证） | 不做路径段 `subprojects/frida-core` |

### L13：函数 / 类 / 命名空间标识符

`profile=deep|explore|abi|full` 在字符串层之后运行 `ApplyIdentifierRenameStrip`：

| 标识符形态 | 映射示例（magic=abcde） |
|------------|-------------------------|
| `frida_agent_main` | `abcde_agent_main` |
| `_frida_*` | `_abcde_*` |
| `Frida` / `FridaScriptEngine` | `Abcde` / `AbcdeScriptEngine` |
| `FRIDA_*` | `ABCDE_*` |
| `namespace Frida` | `namespace Abcde` |

**仍不改**：`re.frida.*` 协议字符串、`Frida.version` 类公开 API 字面量（L11 规则）、仓库目录名 `frida-core`。

入口：

- `StructureAwareRewrite(path, content, magic)` — 单文件分发  
- `ApplyStructureAwareStrip(dir, magic)` — 树遍历  
- deep profile：`ApplyDeepStringLiteralStrip` → 上述结构安全层（Agent/Stub 共用）

## 与「全库字符串 Replace」的区别

| | 裸 `strings.Replace` 全文件 | 本层 |
|--|--|--|
| 区分标识符 vs 字符串 | 否 | 是（词法/AST） |
| 误伤 `void frida_agent_main` | 易 | 否 |
| 版本布局漂移 | 规则可能漏路径 | 语义 token 更稳，但仍依赖策略层规则表 |

策略层（DefaultModOps / DeepModOps）仍负责 **meson 目标名、路径片段、getter 名** 等需要跨文件一致的替换；结构层负责 **「在源文件里改字符串时别砸烂语法/标识符」**。

## 非目标（明确不做）

- 单一 mega-AST 吃掉整个 Frida 树  
- Vala 命名空间 / valac 生成 C 符号的全自动级联重命名  
- 用 AST 层取代二进制静态补丁轨道  

详见 `docs/community-align.md`、`docs/dual-track.md`。
