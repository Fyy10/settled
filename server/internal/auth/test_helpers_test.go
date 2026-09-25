package auth

import (
	"sync"
)

type fillReader struct {
	mu    sync.Mutex
	value byte
	read  int
}

func (reader *fillReader) Read(destination []byte) (int, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()

	for index := range destination {
		destination[index] = reader.value
	}
	reader.read += len(destination)
	return len(destination), nil
}

func (reader *fillReader) bytesRead() int {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	return reader.read
}

type failingReader struct {
	err error
}

func (reader failingReader) Read([]byte) (int, error) {
	return 0, reader.err
}
