package blockedentry

type finalRecovery struct {
	Attempts              uint16 `json:"attempts"`
	ConnectionLoss        uint16 `json:"connection_loss"`
	LaterStarts           uint16 `json:"later_starts"`
	Residuals             uint16 `json:"residuals"`
	AttemptIdentityStable bool   `json:"attempt_identity_stable"`
	DeadlineStable        bool   `json:"deadline_stable"`
}
