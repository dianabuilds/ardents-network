package connection

import (
	"io"
	"sync"
)

func acquireResource(observe func(string, int) uint32, kind string) func() {
	observe(kind, 1)
	var once sync.Once
	return func() { once.Do(func() { observe(kind, -1) }) }
}

func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count == 0 {
			return io.ErrNoProgress
		}
		value = value[count:]
	}
	return nil
}
