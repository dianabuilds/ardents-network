// Package journal owns closed Installation preparation record grammar/order
// and native custody of newly born preparation and transition journals, plus
// independent exact transition custody for explicitly admitted recovery.
// Opaque owners retain original directory/file identities, finite inventory,
// durability and the first failure through physical Close. Transition groups
// hide creation/replacement record handles; Installation retains their schemas
// and phase policy. Creation refuses retained roots; this module
// admits one exact start-attempt and three completion-removal record names as physical provenance;
// their contents and removal ordering remain Installation decisions. A record
// does not attest ACK delivery. OpenTransition independently retains exactly
// observed flat records and an optional independently observed replacement group
// for Installation's admitted recovery. ResyncReplacements synchronizes the
// original group and directory links, including empty groups. It creates no
// phase admission. RetireGenerationFailure resynchronizes an exact first-error
// copy before removing only its original failed phase slot; Installation owns
// failure classification, fresh proofs and subsequent complete-seal admission.
// Neither the copy nor physical retirement grants another Start. It creates no
// original birth ownership or phase authority. The module never authorizes Release, account creation,
// generation selection, process start or replacement.
package journal

import "errors"

var (
	ErrInput       = errors.New("invalid Installation preparation journal")
	ErrBinding     = errors.New("installation preparation journal differs")
	ErrUnavailable = errors.New("installation preparation journal unavailable")
)
