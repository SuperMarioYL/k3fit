[English](README.md) | **简体中文**

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/hero-dark.svg">
  <img src="assets/presentation/hero-light.svg" width="1000" alt="在内置 K3 模型假设下，比较量化层级、context 内存与显存预算。">
</picture>

**在内置 K3 模型假设下，比较量化层级、context 内存与显存预算。**

`v0.2.0` · `Go 1.24+` · [MIT](LICENSE)

[Website](https://k3fit.lei6393.com) · [Demo record](docs/demo-results.json)

## 为什么使用

计划下载和部署大权重前，可以先按明确的模型参数计算权重与上下文内存。K3Fit 将固定状态层、逐 token KV、活动专家和量化系数分开计算，输出每档估计。它的模型常量在源码中标为 schema-unverified，需要与你实际使用的模型核对。

## 架构

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/architecture-dark.svg">
  <img src="assets/presentation/architecture-light.svg" width="1000" alt="model 计算固定状态与 KV 内存；quant 提供 bits-per-weight 表；fit 用显存减去活动权重与固定状态，求最大 context；tps 用固定 240 GB/s 常量除以活动权重。报告保留每档结果，未运行推理。">
</picture>

model 计算固定状态与 KV 内存；quant 提供 bits-per-weight 表；fit 用显存减去活动权重与固定状态，求最大 context；tps 用固定 240 GB/s 常量除以活动权重。报告保留每档结果，未运行推理。

假设在 [spec.go](internal/model/spec.go)，公式在 [delta_attention.go](internal/model/delta_attention.go) 和 [planner.go](internal/fit/planner.go)。--ram 以告警门参与求解：某档位磁盘权重超过内存预算时报告会明确提示，因为 mmap 会从磁盘换页，基于内存带宽的吞吐估计不再成立。

## 安装

需要 Go 1.24+。计算不需要 GPU、模型文件或模型服务。

```bash
git clone https://github.com/SuperMarioYL/k3fit.git
cd k3fit
go build -o k3fit ./cmd/k3fit
```

## 快速开始

两个明确预算场景运行真实 CLI，结果仅验证内置公式的执行。没有核对官方 K3 规格、运行模型或测量吞吐；报告中的倍数与百分比都是给定假设下的计算值。

```bash
go run ./cmd/k3fit --vram 32 --ram 128
go run ./cmd/k3fit --vram 96 --ram 256 --quant Q3_K_M
```

全部输入是命令行预算，命令保存在 [examples/presentation_demo.sh](examples/presentation_demo.sh)。内置 profile 的测试快照在 [testdata/k3spec_golden.json](testdata/k3spec_golden.json)。

## 使用

--vram 和 --ram 为必填 GiB 数；--quant 限制一个层级，--ctx 指定要在报告中检查的 context，--emit-config 输出推荐档位的 llama.cpp 建议启动参数。默认遍历量化表，推荐规则优先选择最大 context，不比较图像或语言质量。

## 实际 Demo

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/process-dark.svg">
  <img src="assets/presentation/process-light.svg" width="1000" alt="两个明确预算场景运行真实 CLI，结果仅验证内置公式的执行。没有核对官方 K3 规格、运行模型或测量吞吐；报告中的倍数与百分比都是给定假设下的计算值。">
</picture>

### 遍历量化表

按 32 GiB 显存预算计算各档结果。

```text
$ go run ./cmd/k3fit --vram 32 --ram 128

K3Fit — Kimi K3 Delta-Attention Fit Planner
══════════════════════════════════════════════════════════
Rig:  32 GiB VRAM | 128 GiB RAM
Model: Kimi K3 — 2.8T params, MoE 896×16, 93 layers (69 Delta-Attention + 24 KV)

Delta-Attention memory at 1M context
+--------------------------------------------------+-------+------------------------------------------+
|                    COMPONENT                     |  GIB  |                  NOTES                   |
+--------------------------------------------------+-------+------------------------------------------+
| DA matrix (69 layers × 2 heads × 128×128 × fp16) |  0.00 | 128×128 matrix/head, context-independent |
| KV cache (24 layers × 1M tokens)                 | 22.89 | standard per-token KV cache              |
| Total (Delta-Attention)                          | 22.89 | at 1M ctx                                |
| (Standard KV — all 93 layers)                    | 88.69 | what generic profilers compute           |
+--------------------------------------------------+-------+------------------------------------------+
Delta-Attention saves 3.9× vs standard KV-cache at 1M context.

Quant fit analysis (VRAM = 32 GiB)
+--------+-------+-------------+---------+---------+-------+
| QUANT  |  BPW  | WEIGHTS GIB | MAX CTX | STD CTX | FITS? |
+--------+-------+-------------+---------+---------+-------+
| Q2_K   |  2.56 |       835.3 |  589837 |    512K |  yes  |
| Q3_K_S |  2.75 |       896.4 |  530709 |    512K |  yes  |
| Q3_K_M |  3.27 |      1065.9 |  366728 |    256K |  yes  |
| Q4_K_S |  3.50 |      1140.9 |  294198 |    256K |  yes  |
| Q4_K_M |  4.25 |      1385.3 |   57687 |     32K |  yes  |
| Q5_K_S |  5.00 |      1629.8 |       — |       — |  no   |
| Q5_K_M |  5.25 |      1711.3 |       — |       — |  no   |
| Q6_K   |  6.10 |      1988.4 |       — |       — |  no   |
| Q8_0   |  8.00 |      2607.7 |       — |       — |  no   |
| F16    | 16.00 |      5215.4 |       — |       — |  no   |
+--------+-------+-------------+---------+---------+-------+
Weights GiB = total model on disk (mmap). Max Ctx = VRAM-resident ceiling.

──────────────────────────────────────────────────────────
Recommendation: Q2_K at 512K context
Expert routing:  16 of 896 experts active per token (1.8% activation)
Predicted decoding tps ≈ 12 (heuristic, ±30% → 8–16)
Disk required:  ~835 GiB (Q2_K GGUF)
VRAM at 512K:      30.5 GiB (weights 18.5 + ctx 12.0) / 32 GiB budget
RAM warning:     weights 835 GiB exceed the 128 GiB RAM budget — mmap will page from disk; the tps estimate assumes RAM-resident weights
1M context:      does NOT fit at any quant — max is 589837 tokens (576K) at Q2_K
```

### 限制量化档

按 96 GiB 显存和 Q3_K_M 再计算。

```text
$ go run ./cmd/k3fit --vram 96 --ram 256 --quant Q3_K_M

K3Fit — Kimi K3 Delta-Attention Fit Planner
══════════════════════════════════════════════════════════
Rig:  96 GiB VRAM | 256 GiB RAM
Model: Kimi K3 — 2.8T params, MoE 896×16, 93 layers (69 Delta-Attention + 24 KV)

Delta-Attention memory at 1M context
+--------------------------------------------------+-------+------------------------------------------+
|                    COMPONENT                     |  GIB  |                  NOTES                   |
+--------------------------------------------------+-------+------------------------------------------+
| DA matrix (69 layers × 2 heads × 128×128 × fp16) |  0.00 | 128×128 matrix/head, context-independent |
| KV cache (24 layers × 1M tokens)                 | 22.89 | standard per-token KV cache              |
| Total (Delta-Attention)                          | 22.89 | at 1M ctx                                |
| (Standard KV — all 93 layers)                    | 88.69 | what generic profilers compute           |
+--------------------------------------------------+-------+------------------------------------------+
Delta-Attention saves 3.9× vs standard KV-cache at 1M context.

Quant fit analysis (VRAM = 96 GiB)
+--------+------+-------------+---------+---------+-------+
| QUANT  | BPW  | WEIGHTS GIB | MAX CTX | STD CTX | FITS? |
+--------+------+-------------+---------+---------+-------+
| Q3_K_M | 3.27 |      1065.9 | 1000000 |      1M |  yes  |
+--------+------+-------------+---------+---------+-------+
Weights GiB = total model on disk (mmap). Max Ctx = VRAM-resident ceiling.

──────────────────────────────────────────────────────────
Recommendation: Q3_K_M at 1M context
Expert routing:  16 of 896 experts active per token (1.8% activation)
Predicted decoding tps ≈ 9 (heuristic, ±30% → 7–12)
Disk required:  ~1066 GiB (Q3_K_M GGUF)
VRAM at 1M:      46.5 GiB (weights 23.6 + ctx 22.9) / 96 GiB budget
RAM warning:     weights 1066 GiB exceed the 256 GiB RAM budget — mmap will page from disk; the tps estimate assumes RAM-resident weights
1M context:      fits at Q3_K_M
```

## 能力与接入

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/integrations-dark.svg">
  <img src="assets/presentation/integrations-light.svg" width="1000" alt="K3Fit 是一个假设可检查的计算器，不读取 GGUF 元数据或实际硬件，也不执行推理。下载与硬件选型前仍需验证模型规格、完整权重驻留需求和运行时开销。">
</picture>

K3Fit 是一个假设可检查的计算器，不读取 GGUF 元数据或实际硬件，也不执行推理。下载与硬件选型前仍需验证模型规格、完整权重驻留需求和运行时开销。



## 配置

量化表位于 internal/quant/table.go，模型常量位于 internal/model/spec.go，带宽常量位于 internal/tps/estimate.go。显示的 ±30% 区间是固定比例而非实测置信区间。运行 --emit-config 可输出推荐档位的 llama.cpp 建议启动参数（模型文件、按档位收敛的 --ctx-size、专家张量固定在 CPU）；使用前请对照 pwilkin/kimi-k3-text fork 当前的参数面核实。暂无拓扑参数。

## 路线图与范围

已实现算术内存分解、量化枚举、context 约束显示、吞吐估计和启动参数导出。设备校准、多卡拓扑和其他模型支持仍为后续方向。

- 内置模型参数明确未完成 schema 验证，不应当作已核实官方规格。
- RAM 以告警门参与 fit（磁盘权重对比 --ram 预算）；吞吐估计仍未读取硬件带宽。
- 这些输出不构成实测性能或部署成功保证。

[Terminal recording](assets/demo.gif) · [Recording script](docs/demo.tape)

## 许可证

[MIT](LICENSE)
