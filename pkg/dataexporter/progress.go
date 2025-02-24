package dataexporter

import (
	"io"
	"sync/atomic"
)

type progressReader struct {
	r            io.Reader
	bytesCounter *atomic.Uint64
}

func newReaderWithBytesCounter(r io.Reader, bytesCounter *atomic.Uint64) *progressReader {
	return &progressReader{
		r:            r,
		bytesCounter: bytesCounter,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.bytesCounter.Add(uint64(n))
	return n, err
}
