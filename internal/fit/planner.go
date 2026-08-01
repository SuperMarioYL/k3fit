// Package fit is the constraint solver: given a K3 spec, VRAM/RAM budget, and
// the quant table, it enumerates (context, quant) pairs and picks the
// maximum-context-that-fits per quant tier.
package fit

import (
	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
)

// StandardContexts are the common power-of-two context lengths (in tokens),
// from 4K up to the K3 maximum of 1M.
var StandardContexts = []int{
	4_096, 8_192, 16_384, 32_768, 65_536,
	131_072, 262_144, 524_288, 1_000_000,
}

// FitResult is the per-quant fit analysis for a given rig.
type FitResult struct {
	Quant            quant.Tier
	MaxContext       int     // exact max tokens that fit in VRAM (0 = weights alone exceed VRAM)
	StandardCtx      int     // largest StandardContexts entry ≤ MaxContext
	WeightsGiB       float64 // total model on disk at this quant
	VRAMWeightsGiB   float64 // active experts + shared, VRAM-resident
	ContextMemGiB    float64 // context memory (DA + KV) at MaxContext
	VRAMTotalGiB     float64 // total VRAM at MaxContext
	TPS              float64 // predicted decoding tps at this quant
	Fits             bool    // MaxContext > 0
	Fits1M           bool    // does 1M context fit?
}

// Plan is the complete sizing plan for a rig.
type Plan struct {
	Spec          model.K3Spec
	VRAMGiB       float64
	RAMGiB        float64
	Results       []FitResult
	Recommended   *FitResult // quant with the highest MaxContext
	Delta1M       model.DeltaMemAccount // memory breakdown at 1M context (quant-independent)
}

// Compute enumerates every quant tier and finds the max-context-that-fits in
// the given VRAM budget.
func Compute(spec model.K3Spec, vramGiB, ramGiB float64, quants []quant.Tier, tpsFn func(quant.Tier) float64) *Plan {
	vramBytes := int64(vramGiB * float64(model.GiB))
	delta1M := model.ComputeDeltaMemAccount(spec, spec.ContextMax)

	plan := &Plan{
		Spec:     spec,
		VRAMGiB:  vramGiB,
		RAMGiB:   ramGiB,
		Delta1M:  delta1M,
		Results:  make([]FitResult, 0, len(quants)),
	}

	var best *FitResult
	bestCtx := -1

	for _, q := range quants {
		// Build a DeltaMemAccount at the K3 max context, then set the quant.
		d := model.ComputeDeltaMemAccount(spec, spec.ContextMax)
		d.SetQuant(q.BytesPerParam())

		vramWeights := d.VRAMWeightsBytes()

		// Max context: VRAM minus weights minus the constant DA matrix, divided
		// by per-token KV cost.
		var maxCtx int
		availForKV := vramBytes - vramWeights - d.DALayerMemBytes
		if availForKV > 0 && d.ContextMemBytesPerToken > 0 {
			maxCtx = int(availForKV / d.ContextMemBytesPerToken)
			if maxCtx > spec.ContextMax {
				maxCtx = spec.ContextMax
			}
		}

		// Context memory at the max context.
		ctxD := model.ComputeDeltaMemAccount(spec, maxCtx)
		ctxD.SetQuant(q.BytesPerParam())

		// 1M fit check.
		d1M := model.ComputeDeltaMemAccount(spec, spec.ContextMax)
		d1M.SetQuant(q.BytesPerParam())
		fits1M := d1M.VRAMTotalBytes() <= vramBytes

		stdCtx := largestStandardCtx(maxCtx)

		fr := FitResult{
			Quant:          q,
			MaxContext:     maxCtx,
			StandardCtx:    stdCtx,
			WeightsGiB:     bytesToGiB(d.QuantWeightsBytes),
			VRAMWeightsGiB: bytesToGiB(vramWeights),
			ContextMemGiB:  bytesToGiB(ctxD.ContextMemBytes),
			VRAMTotalGiB:   bytesToGiB(ctxD.VRAMTotalBytes()),
			TPS:            tpsFn(q),
			Fits:           maxCtx > 0,
			Fits1M:         fits1M,
		}

		plan.Results = append(plan.Results, fr)

		if maxCtx > bestCtx {
			bestCtx = maxCtx
			best = &plan.Results[len(plan.Results)-1]
		}
	}

	plan.Recommended = best
	return plan
}

// largestStandardCtx returns the largest entry in StandardContexts that is ≤ n.
// Returns 0 if none fit.
func largestStandardCtx(n int) int {
	for i := len(StandardContexts) - 1; i >= 0; i-- {
		if StandardContexts[i] <= n {
			return StandardContexts[i]
		}
	}
	return 0
}

func bytesToGiB(b int64) float64 {
	return float64(b) / float64(model.GiB)
}
