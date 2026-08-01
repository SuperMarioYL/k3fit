// Package tps implements the heuristic decoding-tps estimator for Kimi K3.
//
// The estimate is memory-bandwidth-bound: each decoded token must load the
// active-expert weights (16 experts) plus shared layers from wherever they
// are stored (VRAM if they fit, RAM/disk via mmap otherwise) into the compute
// path. The bottleneck is the effective memory bandwidth available for weight
// loading.
//
// tps = effective_bandwidth / active_weight_bytes_per_token
//
// This is a documented heuristic with a ±30% caveat — it is a go/no-go gate
// for a 1.4 TB download, not a benchmark claim. On-device calibration is
// explicitly deferred to v0.2 (out of scope per mvp_plan).
package tps

import (
	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
)

// EffectiveBandwidthGBPS is the single documented calibration constant.
//
// 240 GB/s represents a high-end homelab rig (e.g. RTX 5090 with fast DDR5
// providing partial VRAM caching via mmap). When all active-expert weights
// are resident in VRAM (small models), the true bandwidth approaches the
// GPU's HBM rate (~1800 GB/s on a 5090); when they are mmap'd from RAM
// (the K3 case — 896 experts far exceed VRAM), the effective rate is
// dominated by RAM bandwidth (~100–200 GB/s). 240 GB/s is a blended estimate
// for a well-configured big-iron homelab.
//
// Tuning: replace with an on-device measured value once calibration lands (v0.2).
const EffectiveBandwidthGBPS = 240.0

// Estimate returns the predicted decoding tps for the given K3 spec and
// quant tier. The ±30% caveat applies — see package doc.
func Estimate(spec model.K3Spec, q quant.Tier) float64 {
	// Active weight bytes that must be loaded per token for decoding:
	//   (active experts + shared) × 1e9 × bytes/param
	activeGB := spec.ActivePlusSharedParamsB() * 1e9 * q.BytesPerParam() / 1e9
	if activeGB <= 0 {
		return 0
	}
	return EffectiveBandwidthGBPS / activeGB
}

// Range returns the ±30% bounds for a tps estimate, suitable for display.
func Range(tps float64) (low, high float64) {
	return tps * 0.7, tps * 1.3
}
