package infrastructure

// Source recovery record-sequence grammar (audit IE-7). Every recovery record
// advances the writer/births sequence by one; these constants name the closed
// protocol boundaries the recovery classifier resumes from. The values ARE the
// recovery protocol — changing one changes recovery admission (the R2-2 seam).
// The initial prefix counts account/directory birth records; generation files
// then write at prefix + i*3 (intent) through prefix + i*3 + 2 (ready).
const (
	sourceInitialPrefixSeq       = 68
	sourceInitialSealIntentSeq   = 104
	sourceInitialSealedSeq       = 105
	sourceInitialBindingSeq      = 106
	sourceInitialSelectedSeq     = 109
	sourceInitialUnitSeq         = 110
	sourceInitialReloadIntentSeq = 113
	sourceInitialReloadSeq       = 114
	sourceInitialProjectedCount  = 112

	sourceReplacementPrefixSeq     = 11
	sourceReplacementSealIntentSeq = 47
	sourceReplacementSealedSeq     = 48
	sourceReplacementCount         = 55
)
