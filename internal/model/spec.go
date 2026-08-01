// Package model encodes the published Kimi K3 architecture constants and the
// Delta-Attention memory account — the named, falsifiable structure that
// distinguishes K3 sizing from generic KV-cache profiling.
//
// The spec is marked schema-unverified: constants are best-effort from public
// weight-drop threads and the Delta-Attention architecture description. The
// arithmetic is exact over those constants; the constants themselves may be
// refined as the real K3 spec stabilises.
package model

// GiB is 2^30 bytes — the unit K3Fit uses throughout (matches how VRAM/RAM
// are specified on the command line).
const GiB = 1024 * 1024 * 1024

// K3Spec encodes the Kimi K3 architecture constants used for memory sizing.
//
// Delta-Attention replaces the per-token KV cache in 69 of 93 layers with a
// fixed 128×128 state matrix per head. The remaining 24 layers use standard
// per-token KV cache. This is why a generic profiler (which assumes all 93
// layers use KV) overestimates 1M-context memory by ~3.9×.
type K3Spec struct {
	TotalLayers     int     // 93
	DALayers        int     // 69 — Delta-Attention layers (128×128 matrix/head)
	KVLayers        int     // 24 — standard per-token KV-cache layers
	HeadsPerLayer   int     // GQA KV-head count per layer
	HeadDim         int     // per-head dimension
	DeltaMatrixDim  int     // dimension of the per-head DA state matrix (128)
	ExpertsTotal    int     // 896
	ExpertsActive   int     // 16
	TotalParamsB    float64 // 2800 — total parameter count in billions (2.8T)
	SharedParamsB   float64 // shared (attention + router + non-expert FFN) in billions
	ContextMax      int     // 1_000_000
	BytesPerElement int     // dtype bytes for activations/DA state (fp16 = 2)
}

// DefaultK3Spec returns the best-effort Kimi K3 architecture constants.
func DefaultK3Spec() K3Spec {
	return K3Spec{
		TotalLayers:     93,
		DALayers:        69,
		KVLayers:        24,
		HeadsPerLayer:   2,    // GQA KV heads
		HeadDim:         128,
		DeltaMatrixDim:  128,
		ExpertsTotal:    896,
		ExpertsActive:   16,
		TotalParamsB:    2800, // 2.8T
		SharedParamsB:   12,  // shared attention + router + embeddings
		ContextMax:      1_000_000,
		BytesPerElement: 2, // fp16
	}
}

// ParamsPerExpertB returns the parameter count (in billions) of a single
// expert, derived as TotalParamsB / ExpertsTotal.
func (s K3Spec) ParamsPerExpertB() float64 {
	return s.TotalParamsB / float64(s.ExpertsTotal)
}

// ActiveParamsB returns the parameter count (in billions) of the experts
// active per token (ExpertsActive × ParamsPerExpertB).
func (s K3Spec) ActiveParamsB() float64 {
	return float64(s.ExpertsActive) * s.ParamsPerExpertB()
}

// ActivePlusSharedParamsB returns the parameter count (in billions) that must
// be loaded per token for decoding — active experts + shared layers. This is
// the bandwidth-bound working set that drives the tps heuristic.
func (s K3Spec) ActivePlusSharedParamsB() float64 {
	return s.ActiveParamsB() + s.SharedParamsB
}

// ExpertActivationRatio returns the fraction of experts active per token.
func (s K3Spec) ExpertActivationRatio() float64 {
	return float64(s.ExpertsActive) / float64(s.ExpertsTotal)
}
