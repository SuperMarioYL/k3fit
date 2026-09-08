// Package report renders a fit.Plan as a human-readable table report using
// tablewriter. The output is what `k3fit --vram V --ram R` prints to stdout.
package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SuperMarioYL/k3fit/internal/fit"
	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/tps"
	"github.com/olekukonko/tablewriter"
)

// RenderPlan writes the full K3Fit report for the given plan to w.
func RenderPlan(plan *fit.Plan, w io.Writer) {
	spec := plan.Spec

	// Header
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "K3Fit — Kimi K3 Delta-Attention Fit Planner\n")
	fmt.Fprintln(w, strings.Repeat("═", 58))
	fmt.Fprintf(w, "Rig:  %.0f GiB VRAM | %.0f GiB RAM\n", plan.VRAMGiB, plan.RAMGiB)
	fmt.Fprintf(w, "Model: Kimi K3 — %s params, MoE %d×%d, %d layers (%d Delta-Attention + %d KV)\n",
		fmtParamsT(spec.TotalParamsB), spec.ExpertsTotal, spec.ExpertsActive,
		spec.TotalLayers, spec.DALayers, spec.KVLayers)
	fmt.Fprintln(w)

	// --- Memory breakdown at 1M context ---
	renderMemoryBreakdown(plan, w)

	// --- Per-quant fit table ---
	renderQuantTable(plan, w)

	// --- Recommendation ---
	renderRecommendation(plan, w)
}

func renderMemoryBreakdown(plan *fit.Plan, w io.Writer) {
	d := plan.Delta1M
	spec := plan.Spec

	fmt.Fprintf(w, "Delta-Attention memory at %s context\n", fmtCtx(spec.ContextMax))
	tw := tablewriter.NewWriter(w)
	tw.SetHeader([]string{"Component", "GiB", "Notes"})
	tw.SetAutoWrapText(false)
	tw.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT})

	tw.Append([]string{
		fmt.Sprintf("DA matrix (%d layers × %d heads × %d×%d × fp16)",
			spec.DALayers, spec.HeadsPerLayer, spec.DeltaMatrixDim, spec.DeltaMatrixDim),
		fmt.Sprintf("%.2f", gIB(d.DALayerMemBytes)),
		"128×128 matrix/head, context-independent",
	})
	tw.Append([]string{
		fmt.Sprintf("KV cache (%d layers × %s tokens)", spec.KVLayers, fmtCtx(d.ContextTokens)),
		fmt.Sprintf("%.2f", gIB(d.KVLayerMemBytes)),
		"standard per-token KV cache",
	})
	tw.Append([]string{
		"Total (Delta-Attention)",
		fmt.Sprintf("%.2f", gIB(d.ContextMemBytes)),
		fmt.Sprintf("at %s ctx", fmtCtx(d.ContextTokens)),
	})
	tw.Append([]string{
		fmt.Sprintf("(Standard KV — all %d layers)", spec.TotalLayers),
		fmt.Sprintf("%.2f", gIB(d.StandardKVBytes)),
		"what generic profilers compute",
	})
	tw.Render()

	ratio := d.ReductionRatio()
	fmt.Fprintf(w, "Delta-Attention saves %.1f× vs standard KV-cache at %s context.\n\n",
		ratio, fmtCtx(spec.ContextMax))
}

func renderQuantTable(plan *fit.Plan, w io.Writer) {
	fmt.Fprintf(w, "Quant fit analysis (VRAM = %.0f GiB)\n", plan.VRAMGiB)
	tw := tablewriter.NewWriter(w)
	tw.SetHeader([]string{"Quant", "bpw", "Weights GiB", "Max Ctx", "Std Ctx", "Fits?"})
	tw.SetAutoWrapText(false)
	tw.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_CENTER,
	})

	for _, r := range plan.Results {
		fits := "no"
		if r.Fits {
			fits = "yes"
		}
		stdCtx := "—"
		if r.StandardCtx > 0 {
			stdCtx = fmtCtx(r.StandardCtx)
		}
		maxCtx := "—"
		if r.MaxContext > 0 {
			maxCtx = fmt.Sprintf("%d", r.MaxContext)
		}
		tw.Append([]string{
			r.Quant.Name,
			fmt.Sprintf("%.2f", r.Quant.BPW),
			fmt.Sprintf("%.1f", r.WeightsGiB),
			maxCtx,
			stdCtx,
			fits,
		})
	}
	tw.Render()
	fmt.Fprintln(w, "Weights GiB = total model on disk (mmap). Max Ctx = VRAM-resident ceiling.")
	fmt.Fprintln(w)
}

func renderRecommendation(plan *fit.Plan, w io.Writer) {
	spec := plan.Spec

	if plan.Recommended == nil || !plan.Recommended.Fits {
		fmt.Fprintln(w, "No quant tier fits in the given VRAM. Increase --vram or use a smaller context.")
		return
	}

	r := plan.Recommended
	low, high := tps.Range(r.TPS)

	// Recompute context memory at the recommended standard context (not maxCtx)
	// so the VRAM line reflects what the user will actually run.
	stdD := model.ComputeDeltaMemAccount(spec, r.StandardCtx)
	stdD.SetQuant(r.Quant.BytesPerParam())
	stdCtxMem := gIB(stdD.ContextMemBytes)
	stdVRAMTotal := gIB(stdD.VRAMTotalBytes())

	fmt.Fprintln(w, strings.Repeat("─", 58))
	fmt.Fprintf(w, "Recommendation: %s at %s context\n", r.Quant.Name, fmtCtx(r.StandardCtx))
	fmt.Fprintf(w, "Expert routing:  %d of %d experts active per token (%.1f%% activation)\n",
		spec.ExpertsActive, spec.ExpertsTotal, spec.ExpertActivationRatio()*100)
	fmt.Fprintf(w, "Predicted decoding tps ≈ %.0f (heuristic, ±30%% → %.0f–%.0f)\n",
		r.TPS, low, high)
	fmt.Fprintf(w, "Disk required:  ~%.0f GiB (%s GGUF)\n", r.WeightsGiB, r.Quant.Name)
	fmt.Fprintf(w, "VRAM at %s:      %.1f GiB (weights %.1f + ctx %.1f) / %.0f GiB budget\n",
		fmtCtx(r.StandardCtx), stdVRAMTotal, r.VRAMWeightsGiB, stdCtxMem, plan.VRAMGiB)
	renderRAMWarning(r, plan.RAMGiB, w)

	if r.Fits1M {
		fmt.Fprintf(w, "1M context:      fits at %s\n", r.Quant.Name)
	} else {
		fmt.Fprintf(w, "1M context:      does NOT fit at any quant — max is %d tokens (%s) at %s\n",
			r.MaxContext, fmtCtx(r.MaxContext), r.Quant.Name)
	}
	fmt.Fprintln(w)
}

// renderRAMWarning prints the RAM-budget gate for the chosen tier: when the
// on-disk weights (the mmap working set) exceed the RAM budget, the OS pages
// weights from disk and the RAM-bandwidth bound behind the tps heuristic does
// not hold. Silent when the working set fits.
func renderRAMWarning(r *fit.FitResult, ramGiB float64, w io.Writer) {
	if r.WeightsFitRAM {
		return
	}
	fmt.Fprintf(w, "RAM warning:     weights %.0f GiB exceed the %.0f GiB RAM budget — mmap will page from disk; the tps estimate assumes RAM-resident weights\n",
		r.WeightsGiB, ramGiB)
}

// RenderConstrained writes a report focused on a user-specified --ctx target:
// it finds the best quant tier that fits that context and shows the per-quant
// table filtered to fitting tiers.
func RenderConstrained(plan *fit.Plan, targetCtx int, w io.Writer) {
	spec := plan.Spec

	// Header
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "K3Fit — Kimi K3 Delta-Attention Fit Planner\n")
	fmt.Fprintln(w, strings.Repeat("═", 58))
	fmt.Fprintf(w, "Rig:  %.0f GiB VRAM | %.0f GiB RAM\n", plan.VRAMGiB, plan.RAMGiB)
	fmt.Fprintf(w, "Target: %s context\n", fmtCtx(targetCtx))
	fmt.Fprintf(w, "Model: Kimi K3 — %s params, MoE %d×%d, %d layers (%d Delta-Attention + %d KV)\n",
		fmtParamsT(spec.TotalParamsB), spec.ExpertsTotal, spec.ExpertsActive,
		spec.TotalLayers, spec.DALayers, spec.KVLayers)
	fmt.Fprintln(w)

	// Per-quant table
	fmt.Fprintf(w, "Quant fit for %s context (VRAM = %.0f GiB)\n", fmtCtx(targetCtx), plan.VRAMGiB)
	tw := tablewriter.NewWriter(w)
	tw.SetHeader([]string{"Quant", "bpw", "Weights GiB", "Max Ctx", fmt.Sprintf("Fits %s?", fmtCtx(targetCtx)), "tps"})
	tw.SetAutoWrapText(false)
	tw.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_RIGHT,
	})

	vramBytes := int64(plan.VRAMGiB * float64(model.GiB))
	var bestFit *fit.FitResult

	for i := range plan.Results {
		r := &plan.Results[i]
		// Recompute whether the target ctx fits at this quant.
		d := model.ComputeDeltaMemAccount(spec, targetCtx)
		d.SetQuant(r.Quant.BytesPerParam())
		fits := d.VRAMTotalBytes() <= vramBytes

		fitsStr := "no"
		if fits {
			fitsStr = "yes"
			if bestFit == nil || r.Quant.BPW > bestFit.Quant.BPW {
				bestFit = r
			}
		}
		maxCtx := "—"
		if r.MaxContext > 0 {
			maxCtx = fmt.Sprintf("%d", r.MaxContext)
		}
		tw.Append([]string{
			r.Quant.Name,
			fmt.Sprintf("%.2f", r.Quant.BPW),
			fmt.Sprintf("%.1f", r.WeightsGiB),
			maxCtx,
			fitsStr,
			fmt.Sprintf("%.1f", r.TPS),
		})
	}
	tw.Render()
	fmt.Fprintln(w)

	// Recommendation
	if bestFit == nil {
		fmt.Fprintf(w, "No quant tier fits %s context in %.0f GiB VRAM. Reduce --ctx or increase --vram.\n\n",
			fmtCtx(targetCtx), plan.VRAMGiB)
		return
	}

	low, high := tps.Range(bestFit.TPS)
	fmt.Fprintln(w, strings.Repeat("─", 58))
	fmt.Fprintf(w, "Best quant for %s context: %s\n", fmtCtx(targetCtx), bestFit.Quant.Name)
	fmt.Fprintf(w, "Expert routing:  %d of %d experts active per token (%.1f%% activation)\n",
		spec.ExpertsActive, spec.ExpertsTotal, spec.ExpertActivationRatio()*100)
	fmt.Fprintf(w, "Predicted decoding tps ≈ %.0f (heuristic, ±30%% → %.0f–%.0f)\n",
		bestFit.TPS, low, high)
	fmt.Fprintf(w, "Disk required:  ~%.0f GiB (%s GGUF)\n", bestFit.WeightsGiB, bestFit.Quant.Name)
	renderRAMWarning(bestFit, plan.RAMGiB, w)
	fmt.Fprintln(w)
}

// RenderConfig writes the suggested llama.cpp launch flags for the plan's
// recommended tier, targeting the pwilkin/kimi-k3-text fork. The block is a
// suggestion to verify against the fork's current flag surface — K3Fit never
// touches a GGUF file or a runtime. targetCtx (> 0) is honoured but clamped to
// the tier's fitted MaxContext so the emitted --ctx-size always fits.
func RenderConfig(plan *fit.Plan, targetCtx int, w io.Writer) {
	if plan.Recommended == nil || !plan.Recommended.Fits {
		fmt.Fprintln(w, "No quant tier fits the given VRAM — nothing to emit. Increase --vram or use a smaller context.")
		return
	}

	r := plan.Recommended
	ctx := r.StandardCtx
	if targetCtx > 0 {
		if targetCtx <= r.MaxContext {
			ctx = targetCtx
		} else {
			ctx = r.MaxContext
			fmt.Fprintf(w, "# --ctx %d exceeds the %d-token fit at %s; clamped to the fitted maximum.\n",
				targetCtx, r.MaxContext, r.Quant.Name)
		}
	}

	fmt.Fprintln(w, "# Suggested llama.cpp launch flags (pwilkin/kimi-k3-text fork).")
	fmt.Fprintf(w, "# Fit basis: %s · %s context · VRAM budget %.0f GiB · weights ~%.0f GiB on disk.\n",
		r.Quant.Name, fmtCtx(ctx), plan.VRAMGiB, r.WeightsGiB)
	if !r.WeightsFitRAM {
		fmt.Fprintf(w, "# RAM warning: weights ~%.0f GiB exceed the %.0f GiB RAM budget — mmap will page from disk.\n",
			r.WeightsGiB, plan.RAMGiB)
	}
	fmt.Fprintln(w, "# Verify against the fork's current flags; K3Fit never touches a GGUF or a runtime.")
	fmt.Fprintln(w, "llama-server \\")
	fmt.Fprintf(w, "  --model kimi-k3-text-%s.gguf \\\n", r.Quant.Name)
	fmt.Fprintf(w, "  --ctx-size %d \\\n", ctx)
	fmt.Fprintln(w, "  --n-gpu-layers 999 \\")
	fmt.Fprintln(w, "  --override-tensor \"exps=CPU\"")
}

// --- helpers ---

func gIB(b int64) float64 {
	return float64(b) / float64(model.GiB)
}

// fmtParamsT formats a parameter count in billions as a terse trillions label
// without rounding away significance: 2800B → "2.8T" (not "%.0f" → "3T").
func fmtParamsT(paramsB float64) string {
	return strconv.FormatFloat(paramsB/1000, 'f', -1, 64) + "T"
}

func fmtCtx(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%dM", n/1_000_000)
	}
	if n >= 1024 {
		return fmt.Sprintf("%dK", n/1024)
	}
	return fmt.Sprintf("%d", n)
}
