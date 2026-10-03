package sftp

import (
	"context"
	"io"
)

// Transfer cancellation is checked between chunks, without closing the shared SSH session.
type transferReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r transferReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type transferWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w transferWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.writer.Write(p)
}
