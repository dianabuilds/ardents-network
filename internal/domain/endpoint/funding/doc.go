// Package funding composes finite holder Stock with original Network, Route
// and Hosting owners for Endpoint Source and role openings. It owns serialized
// preparation and issuance ordering, not authority, quota, token verification,
// selection policy or physical Route termination. Prepared rights remain
// unpresented until the genuine Stock.Take at each HELLO. Bootstrap and spent
// rights are never recreated or refunded by failure or cleanup.
//
// Parents owns one installation-wide Entry history and provider budget; its
// original participant closes them only after all Reader/Publisher borrowers
// join. Source/Prefix callers retain physical cleanup and their original
// Execution operation. This package has no CLI, output, worker or legacy owner.
package funding
