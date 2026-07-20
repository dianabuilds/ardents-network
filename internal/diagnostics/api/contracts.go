package api

type EventWriter interface {
	RecordEventCommand(RecordEventCommand) EventEnvelope
}

type Writer interface {
	EventWriter
	BeginOperationCommand(BeginOperationCommand) OperationSnapshot
	CompleteOperationCommand(TransitionOperationCommand)
	FailOperationCommand(TransitionOperationCommand)
	RecoverOperationCommand(TransitionOperationCommand)
	AbandonOperationCommand(TransitionOperationCommand)
	SetPrimaryHealth(SetPrimaryHealthCommand)
	ClearPrimary()
	SetSubsystemHealth(SetSubsystemHealthCommand)
	ClearSubsystem(string)
}

type Service interface {
	DiagnosticsSnapshot() DiagSnapshot
	PendingOperations() []OperationSnapshot
	GetHealthSummary() HealthSnapshot
	ExplainFailure(string, string) FailureExplanationSnapshot
	ListRecentEvents(int, string) ([]EventEnvelope, string)
}
