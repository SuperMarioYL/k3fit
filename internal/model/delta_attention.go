package model

// DeltaMemAccount breaks down Kimi K3's Delta-Attention memory at a given
// context length. The quant-independent fields (DA matrix, KV cache, per-token
// rate) are computed here; the quant-dependent fields (active-expert memory,
// total quantized weights) are filled by the caller (fit.Plan) once a quant
// tier is chosen.
type DeltaMemAccount struct {
	Spec K3Spec

	// ContextTokens is the context length this account was computed at.
	ContextTokens int

	// PerHeadMatrixBytes is the bytes of one 128×128 DA state matrix per head
	// (DeltaMatrixDim² × BytesPerElement).
	PerHeadMatrixBytes int64

	// DALayerMemBytes is the total bytes of the Delta-Attention state across
	// all DA layers — a CONSTANT independent of context length. This is the
	// key difference from standard KV profiling: 69 layers do not grow with
	// context.
	DALayerMemBytes int64

	// KVLayerMemBytes is the total bytes of the per-token KV cache across the
	// 24 standard layers at the given context.
	KVLayerMemBytes int64

	// ContextMemBytesPerToken is the incremental bytes added per token by the
	// 24 KV layers (2 × HeadsPerLayer × HeadDim × BytesPerElement × KVLayers).
	ContextMemBytesPerToken int64

	// ContextMemBytes is the total context memory at the given length:
	// DALayerMemBytes + KVLayerMemBytes.
	ContextMemBytes int64

	// StandardKVBytes is what a generic profiler would compute if it assumed
	// all TotalLayers use standard per-token KV cache (no Delta-Attention).
	// This is the number K3Fit exists to correct.
	StandardKVBytes int64

	// --- quant-dependent fields (filled by fit.Plan, not ComputeDeltaMemAccount) ---

	// ActiveExpertMemBytes is the quantized weight bytes of the active experts
	// (16) that must reside in VRAM for fast expert routing.
	ActiveExpertMemBytes int64

	// SharedMemBytes is the quantized weight bytes of the shared layers
	// (attention, router, embeddings) that reside in VRAM.
	SharedMemBytes int64

	// QuantWeightsBytes is the total model weight bytes at the chosen quant —
	// the on-disk GGUF file size (the mmap source, not a VRAM-resident number).
	QuantWeightsBytes int64
}

// ComputeDeltaMemAccount produces the quant-independent memory breakdown for
// the given K3 spec at the given context length.
func ComputeDeltaMemAccount(spec K3Spec, ctx int) DeltaMemAccount {
	if ctx < 0 {
		ctx = 0
	}

	perHeadMatrix := int64(spec.DeltaMatrixDim) * int64(spec.DeltaMatrixDim) * int64(spec.BytesPerElement)
	daLayers := int64(spec.DALayers) * int64(spec.HeadsPerLayer) * perHeadMatrix

	// Per-token per-KV-layer: 2 (K + V) × heads × head_dim × dtype bytes.
	perTokPerLayer := int64(2) * int64(spec.HeadsPerLayer) * int64(spec.HeadDim) * int64(spec.BytesPerElement)
	kvLayers := int64(spec.KVLayers) * perTokPerLayer * int64(ctx)
	contextPerToken := int64(spec.KVLayers) * perTokPerLayer
	contextMem := daLayers + kvLayers

	// Standard: ALL layers as per-token KV (what ctxprof / llama.cpp would assume).
	standardKV := int64(spec.TotalLayers) * perTokPerLayer * int64(ctx)

	return DeltaMemAccount{
		Spec:                    spec,
		ContextTokens:           ctx,
		PerHeadMatrixBytes:      perHeadMatrix,
		DALayerMemBytes:         daLayers,
		KVLayerMemBytes:         kvLayers,
		ContextMemBytesPerToken: contextPerToken,
		ContextMemBytes:         contextMem,
		StandardKVBytes:         standardKV,
	}
}

// ReductionRatio returns standardKV / deltaTotal — how many times smaller the
// Delta-Attention account is versus a generic KV-cache profiler at this context.
func (d DeltaMemAccount) ReductionRatio() float64 {
	if d.ContextMemBytes == 0 {
		return 0
	}
	return float64(d.StandardKVBytes) / float64(d.ContextMemBytes)
}

// SetQuant fills the quant-dependent fields given a bytes-per-param factor.
func (d *DeltaMemAccount) SetQuant(bytesPerParam float64) {
	d.ActiveExpertMemBytes = int64(float64(d.Spec.ActiveParamsB()) * 1e9 * bytesPerParam)
	d.SharedMemBytes = int64(float64(d.Spec.SharedParamsB) * 1e9 * bytesPerParam)
	d.QuantWeightsBytes = int64(float64(d.Spec.TotalParamsB) * 1e9 * bytesPerParam)
}

// VRAMWeightsBytes returns the VRAM-resident weight bytes (active experts +
// shared layers) at the current quant.
func (d DeltaMemAccount) VRAMWeightsBytes() int64 {
	return d.ActiveExpertMemBytes + d.SharedMemBytes
}

// VRAMTotalBytes returns the total VRAM requirement at the current context and
// quant: VRAM-resident weights + context memory (DA state + KV cache).
func (d DeltaMemAccount) VRAMTotalBytes() int64 {
	return d.VRAMWeightsBytes() + d.ContextMemBytes
}
