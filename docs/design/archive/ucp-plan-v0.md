# Dossier — Plan v0（曾用暂名 UCP）

- Status: 团队对齐草案（zhu × Pedro）；2026-07-03 定名 **Dossier**
- Date: 2026-07-03
- 前置阅读: `claude-code-cr-memory-interface-brief.md`（CR 分析与 hybrid 接口方案）
- 本文为中文工作稿；spec 定稿建议用英文

## 1. 一句话立意

> 每个 entity（个人 / 团队 / 组织）维护唯一一个 context 库：对内是它的长期记忆，对外是它经 policy 过滤后的发言。外界只看到自然语言；协议约束的永远是"我这一侧"怎么存、怎么滤、怎么留痕。

三个决定性性质：

1. **单侧采纳（n=1 即有用）**：对方不需要知道协议存在。一个人装了就获得跨设备记忆 + 披露纪律；双方都装时自动升级（回执 / 失效推送 / 签名 attest）。像 git：本地先有用，远端协作是升级。
2. **一库两面**：披露层不是第二套存储，是同一个库上的 policy 视图。对内 = memory，对外 = view。
3. **Agent-native**：agent 面对的是文件 + 少量 CLI 命令，工作流是 git 的肌肉记忆（edit → check → sync）。

## 2. 与 HCP / CR 的关系（诚实版）

- **HCP** 回答了 why（user-owned、portable、consent-based），停在 position paper。
- **CR** 回答了一半 what：**语义层是对的**（schema / status 生命周期 / provenance / grants / audit），**接口层与拓扑是错的**（重 MCP JSON、cloud-only、单用户）。
- **UCP = 保留 CR 的语义，换掉接口与拓扑**：
  - 文件 + CLI 替代 MCP JSON 工具调用；
  - local-first 多副本替代 cloud-only；
  - entity（人/团队/组织）替代单 user；
  - miss→回源生长替代静态库；
  - 单侧协议替代双端集成。
- 自我定位：**加大加强版 CR**——但"加强"不是功能堆叠，而是换 delivery：把同样的治理保证从 API 形状搬进 agent 已经精通的工具链（lint / CI / review / blame）。CR 用接口强制治理，UCP 用工具链强制治理。

## 3. 架构

```text
        外界（纯自然语言，不需要装协议）
          ↕ 对话
        我的 agent（对话前台）
          · 公共场合 context 默认只装投影 —— 泄不了没有的东西
          ↕ ucp answer
        ucp CLI（确定性闸门与治理，不是 LLM）
          answer / check / sync / review / gc / ask / view / log / grant
          ↕ 读写（过校验才入库）
        一库多副本（local-first，信任边界内）
          电脑=完整副本 · 手机=瘦客户端 · 云=常驻副本(24/7 受理外部请求)
          ↕ miss / 灰区（虚线：回源循环）
        Owner 真人（origin：批灰区、答 miss；答案可入库 → 库沿需求生长）
```

三条铁律：

1. **LLM proposes, policy disposes**：授权判定永远是确定性程序；LLM 只做 NL→slug 映射和向人起草请求。
2. **上下文隔离是主执行机制**：公共场合的 agent 默认只持有投影，要更多必须过闸门。
3. **外部消息全文按不可信输入处理**：prompt injection 的防线在闸门，不在 agent 自觉。

## 4. 数据模型与治理

（本节回应核心担忧：markdown 利好 agent 操作，但用不好会失序。）

### 4.1 原则：表皮是 markdown，骨架是 schema

fact 文件 = YAML frontmatter（结构化字段）+ 受限正文。它是"恰好能被人和 agent 直接读写的**结构化记录**"，不是自由笔记。语法即 schema。

```markdown
<!-- self/profile/dietary.md -->
---
slug: profile.dietary_restrictions
type: array
status: active          # active | superseded | disputed | suggested | archived
sensitivity: normal     # public | normal | sensitive | secret
source: owner           # owner | imported | inferred | peer
confidence: 1.0
updated_at: 2026-07-03
verify_by: 2027-07-03   # 新鲜度契约；过期进 review 队列
supersedes: null        # 或指向被替代的 fact
evidence: kordi/dm-2026-07-01
---
- 花生过敏（严重）
- 不吃内脏
```

### 4.2 两区制：正史区与草稿区

- **正史区**（`self/`、`peers/`）：只收合法 fact 块。`ucp check` fail-closed——不合格式不入库、不 sync、不披露。
- **草稿区**（`notes/`）：自由 markdown，agent 随便写；**永不披露、永不作为事实引用**。园丁定期提名"值得转正的条目"，转成 fact 块才算数。

> 原则：**agent 的自由在边缘，秩序在闸门**。防失序靠机器强制（linter / pre-commit 式），不靠 agent 自觉。

### 4.3 不变量（`ucp check` 强制）

1. 每个 (slug, scope) 至多一个 `active` fact。
2. status 迁移必须合法：`active→superseded` 需 supersedes 指针；`disputed` 一律不可披露。
3. supersedes 链无环。
4. frontmatter 字段类型合法；slug 必须在命名空间登记表内（新 slug = 显式建档操作，不是随手写一个）。
5. `sensitivity: secret` 的值不允许出现在任何投影或 notes。

### 4.4 三个治理问题的机制化回答

**过期（out-of-date）**

- 每类 slug 有默认 TTL → `verify_by`（`calendar.*` 按小时；`profile.address` 一年；`profile.dietary` 基本不动）。
- 两档：**stale**（过 verify_by）仍可披露但带 `as_of` 和 stale 标记，或按 policy 扣住；**expired**（硬过期）视同 unknown，触发回源。
- `ucp review`（园丁）列出过期队列：agent 能从新证据重新确认的自动续期（记 evidence），敏感的进人审。
- 新鲜度永远是从时间戳**算出来**的，不是 LLM 的判断。

**冲突（conflict）**

- **同步冲突**（两台设备改同一 fact）：一事实一文件已把冲突面缩到最小；解决 = 确定性 LWW（updated_at + device id），但败方**不静默丢弃**——进 `conflicts/` 隔离区并列入 review 队列。v0 不做 CRDT。
- **语义冲突**（新旧信息矛盾）：**不覆盖、只替代**——新 fact 带 `supersedes`，旧的转 `superseded` 留史；同 slug 双 active 被 check 直接拒绝。跨 slug 的内容矛盾由园丁（LLM）标记 `disputed` → 人裁决；disputed 不披露。
- 优先级由 provenance 决定：`owner > imported > inferred`；同级比 confidence + 时间。

**膨胀（growth）**

- 规模落在**文件数**而非文件大小；目录 = 命名空间，agent 用 ls/grep 导航——这正是 coding agent 在大 repo 里的日常，也是"多个小文件优于一个大 memory.md"的根本原因。
- 生命周期分层：active（工作集）→ superseded / expired 由 `ucp gc` 移入 `archive/`（默认视图之外），类似 log rotation。
- **对内也走投影**：`ucp view profile` 渲染有界摘要卡。披露闸门与上下文预算是**同一套投影机制的两个用途**（对外 = 隐私，对内 = 省 token）。
- 园丁做近重复合并（LLM 提名、规则/人确认）；命名空间设大小预算，check 超标告警。

### 4.5 治理栈小结（defense in depth）

```text
grammar/schema → 不变量 → sync 门禁 → 园丁巡检 → 人审队列
```

一句话：**乱的进不了正史，草稿写了不算数。**

## 5. 协议面

### 5.1 answer 分类学

`hit-value / hit-generalized / hit-predicate / deny / unknown / pending(escalated)`

响应携带 `as_of`、provenance 摘要、receipt id。

### 5.2 policy.yaml 草案

```yaml
audiences:
  kordi-contacts: { platform: kordi, relation: contact }
  public: {}

defaults:
  on_miss: ask-owner        # unknown | ask-owner
  on_stale: flag            # 带标记披露 | withhold

rules:
  - match: profile.dietary_restrictions
    audience: kordi-contacts
    allow: value
    purpose: [dining, scheduling]
  - match: calendar.*
    audience: kordi-contacts
    allow: predicate         # 只回答 free/busy 类判断
  - match: "*"
    audience: public
    allow: deny
```

policy 文件在库里：**权限即代码**，可 diff、可版本、可 review。

### 5.3 回执与双侧升级

- 每次披露写 ledger（append-only，以云副本侧为准）；owner 有"谁知道我什么"面板。
- 双方都有 UCP 时：receipt 携带回执通道 → 失效/更新推送；披露值可签名 attest。单侧模式下这些静默缺席，不影响可用性。

### 5.4 miss → 回源

库里没有 ≠ 失败，是一等公民：按 policy 返回 `unknown`，或 `ask-owner`（Kordi 里 = 给 owner 弹 DM："B 问 X——回答一次 / 回答并入库？"）。**每次 miss 是需求信号，库沿真实被问的维度生长。** v2 才考虑 connector（日历等系统的可解析引用）。

## 6. 产品形态

```text
~/.ucp/
  self/           # 我的 facts（正史区，按 namespace 分目录、一事实一文件）
  peers/          # 别人披露给我的（带 receipt 指针，可刷新）
  notes/          # 草稿区（不披露、不作数）
  policy.yaml
  ledger/         # 回执只读镜像
  conflicts/      # 同步冲突隔离区
  archive/        # gc 归档
  SKILL.md        # 教 agent 用法（协议的 adoption 载体）
```

CLI（agent 需要会的全部）：

| 命令 | 作用 |
| --- | --- |
| `ucp check` | schema + 不变量校验（fail-closed） |
| `ucp sync` | 副本拉推合并；冲突入隔离区 |
| `ucp answer --to X --purpose P "q"` | 披露闸门：返回分档结果 + 写回执 |
| `ucp ask <peer> "q"` | 反向要信息 → peers/ |
| `ucp review` / `ucp gc` | 园丁队列 / 归档清理 |
| `ucp view <scope>` | 有界投影（对内省 token） |
| `ucp log` / `ucp grant` | 查账 / 管授权 |

分发形态 = **spec + CLI + SKILL.md**（一个 skill 文件夹丢进任何 agent 就会用）。MCP / A2A / HTTP 只是后期的 transport binding，不是协议本体。

## 7. Kordi 旗舰 demo（论文 Figure 1）

A、B 两个用户约饭，B 的 agent 用纯自然语言问 A 的（Kordi 托管）agent：

1. **饮食禁忌**：standing grant 命中 → `hit-value`，回执落账。
2. **时间空档**：`calendar.*` 只允许 predicate → 只回 free/busy，日历细节零泄露。
3. **常去餐厅价位**：库里没有 → `ask-owner`，A 收到 DM 选"回答并入库" → 库生长。

结果：饭订成；A 的日历与健康信息未泄露；ledger 三条回执。一个场景演完全部机制。

## 8. Eval 计划（衔接 brief 的 Harbor）

Arms：

| Arm | 说明 |
| --- | --- |
| markdown-only | 裸 memory.md，无治理（baseline） |
| ucp | 完整：上下文隔离 + 闸门 |
| ucp-无隔离（ablation） | policy 只写在 prompt 里 → 证明"隔离是承重墙" |
| cr-mcp | 沿用 brief，作接口开销对照 |

指标：任务成功率 × 过度披露率 × 注入攻击下泄露率 × tokens/cost → 画 **privacy–utility frontier**。

预期：markdown-only 在隐私轴裸奔；ucp-无隔离在注入下垮掉；完整 ucp 的占优区间清晰。

## 9. Roadmap

- **P0（本周）**：定名；spec 骨架（fact 语法、policy 格式、answer 分类学、不变量表）；SKILL.md 草稿。验收：一个全新 agent 只读 SKILL.md 就能正确操作样例库。
- **P1**：CLI MVP（check / sync / answer / view；git backend；纯本地）。验收：不变量全绿；闸门分档正确；坏文件进不了库。
- **P2**：云常驻副本 + Kordi 双人 demo（§7 三步全通）。
- **P3**：Harbor arm + 数据 → 论文。
- **P4**：双侧升级（回执通道 / 失效推送 / attest）；connector 探索。

CR 侧：brief 的 Phase 1–3 继续作为 owner-plane 实验床；**disclosure plane 按本 spec 新起，不长在 CR 里**。

## 10. 风险（诚实清单）

1. **披露不可撤回**：协议管"给不给"，管不了对方拿去干嘛；purpose / TTL / 回执是契约 + 审计约束，不是技术强制。threat model 边界要写明。
2. **聚合泄露**：多次各自合规的部分披露可以拼图。作为 open problem（disclosure budget 是研究点）。
3. **治理疲劳**：review 队列太吵，人就会无脑全过。园丁必须高精度低打扰（预算：每周人审 < N 条）。
4. **Boil the ocean**：身份 / 同步 / 策略 / 密码学 / agent UX 五个坑，MVP 按 §9 残忍执行。
5. **命名撞车**：见 §12。

## 11. Open questions（找 Pedro 对齐）

1. 名字定哪个（§12）。
2. org/team 库的优先级：MVP 锁个人，数据模型不堵路（per-fact provenance + 库内角色）即可——对外叙事要不要先提？
3. Kordi 耦合深浅：demo 用 Kordi，spec 不依赖 Kordi——确认？
4. spec 语言（建议英文）与开源节奏。
5. CR 仓库继续投入多少（brief Phase 1–3）vs 全力新起。

## 12. 命名

"UCP" 与 Google/Shopify 的 Universal Commerce Protocol（2026-01 发布，ucp.dev，已 8000+ 商家）撞车，对外必须换。候选（已做初步撞名检查，2026-07-03）：

| 候选 | 隐喻 | 撞名情况 | 判定 |
| --- | --- | --- | --- |
| **Dossier** | 档案：关于一个人的结构化事实集；"自己的档案自己管"是天然叙事；中文"档案"共鸣强 | 仅医疗能力管理、车队管理等不相关领域；AI/dev/协议圈干净 | **推荐** |
| Aperture | 光圈：档位 = 披露梯度（原值/泛化/谓词） | Apple Aperture 已死；小型 crypto | 备选 |
| Shoji | 障子纸门：透光不透形 | 近乎无（一个小众 Wayland compositor） | 备选 |
| Facet | 宝石切面 | Facet.ai（被 Adobe 收购）、Facets.cloud、FACET prompt 语言——AI 圈太挤 | 弃 |
| Limen | 门槛 / liminal | Meirtz/Limen（agent 协调 + audit，领域太近）、limenauth.dev | 弃 |
| Janus / 门神 Menshen | 双面门神 / 守门神，概念完美 | JanusGraph、Janus WebRTC / 无 | 内部代号 |

**已定（2026-07-03）**：对外 **Dossier**（"Dossier: a single-sided protocol for governed disclosure of personal context"），CLI `dossier`（或别名 `dsr`），内部代号随意（门神）。待办：域名（dossier.sh / dossierprotocol.org 等）、npm/pypi 包名、商标粗查；plan v0.1 时全文改名并重命名本文件。
