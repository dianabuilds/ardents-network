package entry

import (
	"net"
	"strconv"
)

func (owner *owner) validate(raw []byte) (invite, Candidate, Class, error) {
	return validateInvite(raw, Verification{Current: owner.config.Current, Conflict: owner.config.Conflict,
		Clock: owner.config.Clock, TimeConfident: owner.config.TimeConfident})
}

func candidateByKey(view View, keyID [32]byte) (Candidate, bool) {
	if len(view.Candidates) == 0 || len(view.Candidates) > 64 {
		return Candidate{}, false
	}
	for _, candidate := range view.Candidates {
		if candidate.KeyID == keyID && candidate.PublicKey != [32]byte{} {
			return candidate, true
		}
	}
	return Candidate{}, false
}

func validEndpoint(value string) bool {
	host, port, err := net.SplitHostPort(value)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}
