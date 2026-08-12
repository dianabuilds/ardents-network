package epoch

import "errors"

func verifyEpochChain(current *Snapshot, epoch epochEnvelope) error {
	if epoch.number > maximumEpochChain {
		return errors.New("epoch exceeds the retained chain bound")
	}
	var zero [32]byte
	if current == nil {
		if epoch.number != 1 || epoch.previous != zero {
			return errors.New("genesis epoch chain is invalid")
		}
		return nil
	}
	if epoch.number != current.Epoch+1 || epoch.previous != current.Digest {
		return errors.New("epoch transition does not extend current state")
	}
	return nil
}
