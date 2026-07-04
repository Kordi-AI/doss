# 记忆兼容层设计草案（讨论稿）

- Date: 2026-07-04 · Status: 待讨论（issue #8）
- 一句话：**不和 memory 系统竞争，做"让任何 memory 系统变得可治理"的一层。**

## 定位

现有 memory 工作（Mem0、Letta/MemGPT、Zep、LangMem…）全部在优化同一件事：**你自己的 agent 回忆得更准更省**。没有任何一家做"**别人来要信息时怎么办**"。这就是我们的新场景：治理 + 同步 + 披露。记忆本身我们不必赢，兼容即可——他们负责记得好，我们负责管得住。

## 架构：三层，文件永远是老大

```text
┌─ 生态 bridges（可选）      Mem0 / Letta / Zep ⇄ 文件（带 provenance 落库）
├─ 检索加速（可选）          .index/：SQLite FTS（+可选 embeddings），由文件增量构建
└─ canonical 文件（永远）    self/ peers/ notes/ —— check、sync、policy、answer 只认这里
```

三条不可妥协的不变量：

1. **文件夹是唯一事实源**。权限、check、sync、披露全部只认文件；任何 adapter 坏掉/删掉，库完好无损。
2. **索引是衍生缓存**。`.index/` 可随时重建、gitignore、永不进云——所以它不参与安全模型。
3. **bridge 进来的信息按 provenance 落库**：外部系统写入的一律 `source: imported`（带 evidence 指向来源），推断类标 `suggested` 走确认流程。bridge 不能绕过 check，也永远碰不到披露路径。

## 候选选型（按"差异要大"原则）

| 档位 | 方案 | 特点 | 依赖 |
| --- | --- | --- | --- |
| 极轻（默认） | plain：ls/grep/read | 零依赖、零索引，agent 原生技能 | 无 |
| 高性能（内置） | indexed：SQLite FTS5 + 可选本地 embeddings | 毫秒全文/语义检索，从文件增量构建 | 单文件 SQLite |
| 生态最大 | bridge: Mem0 | 用户已有 Mem0 记忆可双向导入导出 | Mem0 API |
| 研究前沿 | bridge: Letta (MemGPT) | agentic 自编辑记忆，学术对话强 | Letta server |
| 企业稳定（观察） | bridge: Zep | 时序知识图谱，商业化成熟 | Zep 服务 |

推荐组合：**plain 默认 + indexed 内置**（这两个不引入任何外部服务，符合"一个文件夹+一个小程序"），bridge 先做 **Mem0**（生态最大、API 简单），第二个做 **Letta**（论文对话价值），Zep 观察。

## Bridge 的方向性（待定）

- **pull（推荐先做）**：外部系统 → 文件。单向、简单、安全（进来必过 check + provenance）。
- **push**：文件 → 外部系统。等于把库喂给第三方服务，隐私上要过 policy（可以复用 give 档位！`bridge push` 视作一个 requester）——这个设计很优雅但先不做。

## 给论文的表述

> Dossier is not another memory system; it is the governance layer that any memory system can sit behind. Memory work optimizes recall for the self; Dossier governs disclosure to others — an orthogonal, previously unaddressed axis.

## 早上要拍板的

1. 推荐组合（plain + indexed 内置，bridge 先 Mem0 后 Letta）同意吗？
2. bridge 先只做 pull 方向，push 走 policy 复用 give 档位——同意吗？
3. embeddings 默认关（隐私：不把内容发给 embedding API；本地模型才默认开）——同意吗？
4. Claude Code 的 auto-memory（本身就是文件夹）算"天然兼容"案例，README 里要不要点名？
