# Dossier

> 一个带同步的记忆文件夹 + 一道只在信息出门时启动的关卡。
> Your agent's memory, your rules.

对 agent：纯 md/yaml 文件的长期记忆——记 = 写文件，想 = 读文件，零仪式。
对外界：别人来要信息时，本地小程序按 `policy.yaml` 决定给多少：**可以说的 / 别说 / 问主人**，每笔记账。
对方不需要装任何东西；一个人用就有用（多设备同步 + 披露纪律）。

状态：v0 开发中 · [设计文档](docs/design/plan-v0.1.md) · [Roadmap issues](https://github.com/Kordi-AI/dossier/issues)

## 安装

**方式一：自己装**

```sh
git clone https://github.com/Kordi-AI/dossier && cd dossier && ./install.sh
dossier init --github   # 顺手建一个你自己的私有 GitHub 仓库当云端；纯本地就去掉 --github
```

**方式二：发给你的 agent，让它替你装**

> 请帮我安装 Dossier 并初始化记忆库：
> 1. `git clone https://github.com/Kordi-AI/dossier && cd dossier && ./install.sh`
> 2. 运行 `dossier init --github`（我没有 gh 或不想上云就用 `dossier init`）
> 3. 读一遍 `~/.dossier/SKILL.md`，之后照它的规则替我管理记忆。

## 用法

装完之后 agent 只需要知道四件事（详见生成的 `~/.dossier/SKILL.md`）：

| 时机 | 做什么 |
| --- | --- |
| 学到持久信息 | 写进 `self/` 下的小文件，路径即主题 |
| 需要回忆 | 直接 ls / grep / 读文件 |
| 改完文件 | `dossier check --changed`（报错精确到行，修完重跑） |
| 告一段落 | `dossier sync`（未过检的内容永远不会被同步） |

## 架构

![Dossier 架构](docs/architecture.png)

## 文档

- [Plan v0.1](docs/design/plan-v0.1.md) — 全部设计决策（含决策记录表）
- [记忆兼容层草案](docs/design/memory-adapters.md) — 不竞争 memory 系统，兼容它们
- [历史文档](docs/design/archive/)
