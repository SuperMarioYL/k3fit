// Package report renders a fit.Plan as a human-readable table report using
// tablewriter. The output is what `k3fit --vram V --ram R` prints to stdout.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/SuperMarioYL/k3fit/internal/fit"
	"github.com/SuperMarioYL/k3fit/internal/model"
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
	fmt.Fprintf(w, "Model: Kimi K3 — %.0fT params, MoE %d×%d, %d layers (%d Delta-Attention + %d KV)\n",
		spec.TotalParamsB/1000, spec.ExpertsTotal, spec.ExpertsActive,
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
	low, high := rangeBounds(r.TPS)

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

	if r.Fits1M {
		fmt.Fprintf(w, "1M context:      fits at %s\n", r.Quant.Name)
	} else {
		fmt.Fprintf(w, "1M context:      does NOT fit at any quant — max is %d tokens (%s) at %s\n",
			r.MaxContext, fmtCtx(r.MaxContext), r.Quant.Name)
	}
	fmt.Fprintln(w)
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
	fmt.Fprintf(w, "Model: Kimi K3 — %.0fT params, MoE %d×%d, %d layers (%d Delta-Attention + %d KV)\n",
		spec.TotalParamsB/1000, spec.ExpertsTotal, spec.ExpertsActive,
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

	low, high := rangeBounds(bestFit.TPS)
	fmt.Fprintln(w, strings.Repeat("─", 58))
	fmt.Fprintf(w, "Best quant for %s context: %s\n", fmtCtx(targetCtx), bestFit.Quant.Name)
	fmt.Fprintf(w, "Expert routing:  %d of %d experts active per token (%.1f%% activation)\n",
		spec.ExpertsActive, spec.ExpertsTotal, spec.ExpertActivationRatio()*100)
	fmt.Fprintf(w, "Predicted decoding tps ≈ %.0f (heuristic, ±30%% → %.0f–%.0f)\n",
		bestFit.TPS, low, high)
	fmt.Fprintf(w, "Disk required:  ~%.0f GiB (%s GGUF)\n\n", bestFit.WeightsGiB, bestFit.Quant.Name)
}

// --- helpers ---

func gIB(b int64) float64 {
	return float64(b) / float64(model.GiB)
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

func rangeBounds(tps float64) (float64, float64) {
	return tps * 0.7, tps * 1.3
}
