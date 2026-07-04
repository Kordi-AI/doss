# Dossier

> 一个带同步的记忆文件夹，加一道只在信息出门时才启动的关卡。
> A single-sided protocol for governed disclosure of personal context.

状态：设计阶段，plan v0.1 已定稿 · private

## 它是什么

- 对 agent：一个纯 md/yaml 的长期记忆文件夹——记东西 = 写文件，想东西 = 读文件，零仪式、零等待。
- 对外界：别人（人或 agent）用纯自然语言来要信息时，一个本地小程序按规则决定给多少：**可说的内容 / 别说 / 我去问主人**，每笔记账。
- 对方不需要装任何东西：单侧协议，一个人用就有用（跨设备记忆 + 披露纪律）；双方都用时自动升级（回执、更新推送）。

## 架构

![Dossier 架构](docs/architecture.png)

（矢量版：[docs/architecture.svg](docs/architecture.svg)）

## 设计原则

1. **Efficiency 只看热路径**：agent 每次读写记忆必须是纯文件操作。md/yaml 是唯一 agent 界面，MCP/A2A 最多是以后的外接方式。
2. **Capability 放冷路径**：后台、出口、一次性配置处不手软——机器判的错写入当场退回重试；语义问题（重复/矛盾/过期）脏度过阈值后借下次写入触发清理；信息出门是硬门槛。
3. **规则钉在环境里，不靠 agent 记性**：对外场合 agent 手里只有名片级信息（说不出没有的东西）；每次交互的返回自带规则提醒。能力可能漂移，安全不会漂移。
4. **技术栈**：一个文件夹 + 一个单二进制小程序 + git。没有数据库、没有服务框架。

## 文档

| 文件 | 说明 |
| --- | --- |
| [dossier-plan-v0.1.md](dossier-plan-v0.1.md) | 当前 plan（含已定决策记录，共九轮设计讨论的结论） |
| [ucp-plan-v0.md](ucp-plan-v0.md) | 历史版本（项目曾暂名 UCP，因与 Google/Shopify 的 Universal Commerce Protocol 撞名弃用） |
| [claude-code-cr-memory-interface-brief.md](claude-code-cr-memory-interface-brief.md) | 前置研究：Context Router 分析与 markdown 接口方案 |

## Roadmap

见 Issues：P0 记忆层 → P1 出门关卡（Kordi 实测）→ P2 自托管常驻 → P3 评测与论文；v1+ 后置清单另开一条。

## 定位

对标 long-term memory 系统（Mem0 / Letta / Zep）：先证明文件式记忆在标准 benchmark 上打平且大幅省 token，再亮所有记忆系统都没有的绝活——受规则约束、可审计的对外披露。

血缘：HCP（hcp.me）回答了 why，Context Router 验证了语义层，Dossier 换掉接口与拓扑——把治理搬进 agent 最熟的工具链（文件 + git + lint 式检查）。
