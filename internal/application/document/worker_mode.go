package document

// WorkerMode fixes the installed worker entrypoint's responsibility.
type WorkerMode uint8

const (
	ReaderWorker     WorkerMode = 1
	PublisherWorker  WorkerMode = 2
	ByteStreamWorker WorkerMode = 3
)
