package serviceconn

import "errors"

var errNameBindingChanged = errors.New("resolved Service Name binding changed")

func validateNameOrigin(input Request, credential Credential) error {
	if input.NameBinding == (DestinationBinding{}) {
		if input.NameUpdates != nil {
			return errors.New("name updates exist without resolved provenance")
		}
		return nil
	}
	if input.NameBinding.Target != credential.Target || input.NameUpdates == nil ||
		input.NameBinding.Name == "" || input.NameBinding.Generation == 0 || input.NameBinding.Revision == 0 ||
		input.NameBinding.RecordDigest == [32]byte{} || input.NameBinding.Commitment == [32]byte{} {
		return errors.New("resolved Service Name provenance is incomplete")
	}
	if input.OpenAttachment != nil && input.RecoveryBinding.DestinationBinding != input.NameBinding.Commitment {
		return errors.New("recovery destination does not bind the resolved Service Name")
	}
	return nil
}

func (stream *recoveryStream) watchNameOrigin() {
	if stream.nameBinding == (DestinationBinding{}) {
		return
	}
	go func() {
		for {
			select {
			case <-stream.done:
				return
			case update, ok := <-stream.nameUpdates:
				if !ok || !continuesNameBinding(stream.nameBinding, update) {
					stream.fail(errNameBindingChanged)
					return
				}
			}
		}
	}()
}

func continuesNameBinding(initial, update DestinationBinding) bool {
	if update == (DestinationBinding{}) || update.Name != initial.Name || update.Generation != initial.Generation ||
		update.Revision < initial.Revision || update.Authority != initial.Authority || update.Target != initial.Target ||
		update.ParentName != initial.ParentName || update.ParentGeneration != initial.ParentGeneration {
		return false
	}
	return update.Revision > initial.Revision || update.RecordDigest == initial.RecordDigest
}
