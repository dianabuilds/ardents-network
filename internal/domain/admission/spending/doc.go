// Package spending owns the receiving duty's durable token-spend history.
// One exclusive lease binds it to Network, profile, Node and duty generation.
// A token is recorded before work is admitted. Introduction registration slots,
// physical capacity and transport lifetime belong to their respective owners.
// Successful fresh durable creation may issue one opaque initialization fact
// before any receiving attempt; retained empty history never recreates that fact.
// OpenRetained requires both existing native files and complete history, without
// initialization or crash-tail repair; it is the strict explicit-restart seam.
// Linux Inspection separately retains complete original history read-only under
// its existing exclusive lease and numeric principal. It never initializes,
// repairs, prunes, spends or lends runtime authority; Close joins original reads
// and physical closes before returning that lease.
package spending
