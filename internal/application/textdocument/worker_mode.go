package textdocument

// WorkerMode fixes the installed worker entrypoint's responsibility.
type WorkerMode uint8

const (
	ReaderWorker    WorkerMode = 1
	PublisherWorker WorkerMode = 2
)
