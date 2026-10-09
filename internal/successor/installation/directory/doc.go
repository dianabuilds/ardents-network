// Package directory owns physical mutable-directory birth and private original
// inode observations for a serialized Linux creation sequence. It retains the
// original caller, exclusively creates paths selected by Installation, promotes
// the same inode to the admitted non-root account/mode, synchronizes creation,
// and refuses foreign or substituted roots. Identity returns detached facts.
// Independent inspection may use Matches without acquiring creation provenance.
// It owns no account selection, Release proof, writer lease, journal/phase,
// generation binding, adoption/recovery policy or startup authority.
package directory
