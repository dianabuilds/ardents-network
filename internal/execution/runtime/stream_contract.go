package runtime

import "errors"

// StreamContract is a checked directional byte contract from trusted workload
// composition, bound to the original qualified operation. It grants neither
// Service authentication nor another worker invocation. Copies preserve the
// same immutable limits and original authority; peers cannot construct one.
type StreamContract struct {
	operation *Operation
	send      uint64
	receive   uint64
}

// BindStreamContract admits the workload's original limits, not peer fields.
// Ordinary text composition supplies its request/response bounds; trusted
// qualification composition may supply the existing 64MiB directional bound.
// Once bound, this operation cannot select a different contract for recovery.
func (operation *Operation) BindStreamContract(send, receive uint64) (StreamContract, error) {
	if err := operation.Check(); err != nil {
		return StreamContract{}, err
	}
	if send == 0 || receive == 0 || send > 64<<20 || receive > 64<<20 {
		return StreamContract{}, errors.New("execution stream byte contract is invalid")
	}
	operation.contractMu.Lock()
	defer operation.contractMu.Unlock()
	if operation.sendLimit != 0 && (operation.sendLimit != send || operation.receiveLimit != receive) {
		return StreamContract{}, errors.New("original execution stream byte contract differs")
	}
	if err := operation.Check(); err != nil {
		return StreamContract{}, err
	}
	operation.sendLimit, operation.receiveLimit = send, receive
	return StreamContract{operation: operation, send: send, receive: receive}, nil
}

// Limits reobserves the exact original operation. Detached copied numbers and
// a contract from another operation cannot authorize byte admission.
func (contract StreamContract) Limits(operation *Operation) (uint64, uint64, error) {
	if operation == nil || contract.operation != operation {
		return 0, 0, errors.New("original execution stream contract is absent")
	}
	if err := operation.Check(); err != nil {
		return 0, 0, err
	}
	operation.contractMu.Lock()
	defer operation.contractMu.Unlock()
	if contract.send == 0 || contract.receive == 0 || operation.sendLimit != contract.send || operation.receiveLimit != contract.receive {
		return 0, 0, errors.New("original execution stream contract differs")
	}
	return contract.send, contract.receive, operation.Check()
}
