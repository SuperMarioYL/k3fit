<div align="right"><sub><a href="./README.md">EN</a>&nbsp;&nbsp;⇄&nbsp;&nbsp;<b>中文</b></sub></div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/hero-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/hero-light.svg">
  <img src="./assets/hero-light.svg" width="880" alt="K3Fit">
</picture>

<p align="center"><sub>K3Fit 是一个 Go CLI，在下载 1.4&nbsp;TB 之前为你的机器测算 <b>Kimi K3</b> 的适配方案。</sub></p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="license"></a>
  <a href="https://github.com/SuperMarioYL/k3fit/releases"><img src="https://img.shields.io/github/v/release/SuperMarioYL/k3fit" alt="release"></a>
  <img src="https://img.shields.io/github/actions/workflow/status/SuperMarioYL/k3fit/ci.yml?label=CI" alt="CI">
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Kimi%20K3-fit%20planner-5E5CE6" alt="Kimi K3">
</p>

**在下载 1.4&nbsp;TB 的 Kimi K3 之前，先跑这一条命令。**

K3Fit 是一个单命令 Go CLI，使用 Kimi K3 的 Delta-Attention 内存模型来选择你的最大上下文、GGUF 量化等级和专家路由配置——并在你承诺 1.4&nbsp;TB 下载之前报告预测的 tps，让你的 Agent 工作流和家庭实验室设备一次就能正确配置。

<h2><img src="https://api.iconify.design/tabler:topology-star-3.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 架构</h2>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/atlas-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/atlas-light.svg">
  <img src="./assets/atlas-light.svg" width="880" alt="K3Fit 流水线：CLI 参数 → Delta-Attention 账户 → 量化+适配求解 → tps+报告">
</picture>

K3Fit 是纯算术运算，基于固定的 K3 架构规格 + GGUF 量化表——单进程，无网络，无模型文件。核心原语是 **Delta-Attention 内存账户**：93 层中有 69 层将每 token 的 KV 缓存替换为每头 128×128 的固定状态矩阵，因此 1M 上下文内存配置比通用 KV 缓存分析器小 3.9 倍。

<h2><img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 安装与快速开始</h2>

```bash
go install github.com/SuperMarioYL/k3fit/cmd/k3fit@latest
k3fit --vram 32 --ram 128
```

或从源码构建：

```bash
git clone https://github.com/SuperMarioYL/k3fit.git
cd k3fit && go build -o k3fit ./cmd/k3fit
./k3fit --vram 32 --ram 128
```

<details><summary>示例输出</summary>

```
K3Fit — Kimi K3 Delta-Attention Fit Planner
══════════════════════════════════════════════════════════
Rig:  32 GiB VRAM | 128 GiB RAM
Model: Kimi K3 — 2.8T params, MoE 896×16, 93 layers (69 Delta-Attention + 24 KV)

Delta-Attention memory at 1M context
  DA matrix (69 layers)     0.00 GiB  fixed
  KV cache (24 layers)    22.89 GiB  per-ctx
  Total (Delta-Attention) 22.89 GiB
  (Standard KV all 93)   88.69 GiB  — what generic profilers compute
  Delta-Attention saves 3.9× at 1M context.

──────────────────────────────────────────────────────────
Recommendation: Q2_K at 512K context
Expert routing:  16 of 896 experts active per token (1.8% activation)
Predicted decoding tps ≈ 12 (heuristic, ±30% → 8–16)
Disk required:  ~835 GiB (Q2_K GGUF)
VRAM at 512K:  30.5 GiB / 32 GiB budget
1M context:  does NOT fit at any quant — max is 589837 tokens (576K) at Q2_K
```

</details>

<h2><img src="https://api.iconify.design/tabler:terminal-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 用法</h2>

```bash
# 完整适配 + 量化 + 路由 + 预测 tps
k3fit --vram 32 --ram 128

# 约束到特定量化等级
k3fit --vram 96 --ram 256 --quant Q3_K_M

# 检查特定上下文大小
k3fit --vram 32 --ram 128 --ctx 512000
```

| 参数 | 类型 | 默认值 | 含义 |
|---|---|---|---|
| `--vram` (`-v`) | GiB | 必填 | 显存预算 |
| `--ram` (`-r`) | GiB | 必填 | 系统内存预算（mmap 工作集） |
| `--quant` (`-q`) | string | 全部等级 | 约束到单个 GGUF 量化等级（如 `Q2_K`、`Q3_K_M`、`Q4_K_M`） |
| `--ctx` | int | 0（自动） | 目标上下文长度（token 数） |

<h2><img src="https://api.iconify.design/tabler:photo.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 演示</h2>

<img src="./assets/demo.gif" width="880" alt="k3fit --vram 32 --ram 128 输出">

由 `docs/demo.tape` 通过 [vhs](https://github.com/charmbracelet/vhs) 渲染——见 `.github/workflows/demo.yml` 重新渲染。

<h2><img src="https://api.iconify.design/tabler:bulb.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 为什么做这个</h2>

Kimi K3 是一个 1.4&nbsp;TB、2.8&nbsp;T 参数的 MoE 模型，采用全新的 Delta-Attention 架构：93 层中有 69 层将每 token 的 KV 缓存替换为每头 128×128 的状态矩阵，将 1M 上下文内存从 ~89&nbsp;GiB 降至 ~23&nbsp;GiB。想要本地运行的运营者目前必须 (a) 寻找分叉的 llama.cpp 分支，因为上游不支持 Delta-Attention，(b) 盲目选择 GGUF 量化等级，(c) 在 1.4&nbsp;TB 下载完成后才发现预热卡顿问题——无法预测自己的设备是否能容纳所选的上下文 + 量化 + 专家路由。

K3Fit 是通用工具无法建模的 Delta-Attention 适配分析器——像 ctxprof 这样的通用分析器对 K3 计算的是错误的标准 KV 数字，而 [`pwilkin/kimi-k3-text`](https://github.com/pwilkin/llama.cpp/tree/kimi-k3-text) 分支提供推理但不提供适配分析。Kimi K3 部署的兴趣是具体且不断增长的——见 [`diegosouzapw/OmniRoute`](https://github.com/diegosouzapw/OmniRoute) 和 r/LocalLLaMA 部署帖子——但没有工具在下载之前告诉你你的设备能否运行它。

### 对比通用工具

| 功能维度 | K3Fit | stock llama.cpp / Ollama | ctxprof | GrEarl GGUF 包 |
|---|:---:|:---:|:---:|:---:|
| Delta-Attention 内存模型 | ✓ | — | — | — |
| 按量化等级的最大上下文适配 | ✓ | — | 部分 | — |
| 预测解码 tps | ✓ | — | — | — |
| 专家路由配置 | ✓ | — | — | — |
| 无需下载模型 | ✓ | — | ✓ | — |
| 运行 K3 推理 | — | ✓（分叉） | — | —（仅权重） |

K3Fit 是适配工具，不是运行时——llama.cpp 仍然是今天实际运行 K3 的唯一路径。K3Fit 在你开始之前就告诉你那 1.4&nbsp;TB 的下载是否值得。

<h2><img src="https://api.iconify.design/tabler:map-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 路线图</h2>

- [x] **m1 — Delta-Attention 内存账户**：按组件分解（DA 矩阵 × 69 层 + KV × 24 层 + 活跃专家 + 量化权重），按量化等级的最大适配上下文，±30% 启发式 tps
- [x] **m2 — 预测 tps + 约束参数**：带宽受限的 tps 估算，`--quant` / `--ctx` 参数
- [ ] **m3 — 输出配置 + 演示**：`--emit-config` 写出 llama.cpp 分叉启动参数，精修终端演示
- [ ] 设备端 tps 校准（v0.2）——用实测值替换启发式常量
- [ ] 多 GPU / 张量并行拓扑规划
- [ ] 非 K3 模型（Llama、Qwen、Mistral 适配）

### 分享

```
K3Fit — the Go CLI that sizes Kimi K3 for your rig before a 1.4TB download. Delta-Attention-aware memory math, quant fit, predicted tps, no GPU touch. https://github.com/SuperMarioYL/k3fit
```

<h2><img src="https://api.iconify.design/tabler:license.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 许可证</h2>

MIT——见 [LICENSE](./LICENSE)。在 [GitHub 仓库](https://github.com/SuperMarioYL/k3fit/issues) 提交 issue 或 PR。

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
