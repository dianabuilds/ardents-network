// Package selection owns independently leased Route selection roots.
// Installation Entry pairs retain their existing canonical root and six-hour
// bounds. Each local role has a separate persisted Interior pair and original
// thirty-minute horizon. A fresh Network RuntimeView supplies every live check;
// roots, receipts and copied participant values never grant current authority.
// Context-local Rendezvous choices retain their original set and bound across
// physical leg replacement; failures and new exclusions cannot redraw them.
// Entry-pair eligibility/draw/bounds, retained Leg checks and recipient selection
// are portable rules. entry_set.go owns no filesystem, lease or live authority.
// The Linux installation/selection owner separately owns leased durable roots;
// its platform constraint does not constrain checking an already retained Leg.
package selection
