package diagnostics

import core "ardents/internal/diagnostics/recorder"

func New(path string) *Recorder {
	return core.New(path)
}

func NewInDir(dir string) *Recorder {
	return core.NewInDir(dir)
}
