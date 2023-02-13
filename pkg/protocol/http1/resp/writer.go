package resp

import (
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
)

type chunkedBodyWriter struct {
	wroteHeader bool
	r           *protocol.Response
	w           network.Writer
}

// Write will encode chunked p before writing
// It will only return the length of p and a nil error if the writing is successful or 0, error otherwise.
func (c *chunkedBodyWriter) Write(p []byte) (n int, err error) {
	if !c.wroteHeader {
		if err = WriteHeader(&c.r.Header, c.w); err != nil {
			return
		}
		c.wroteHeader = true
	}
	if err = ext.WriteChunk(c.w, p); err != nil {
		return
	}
	return len(p), nil
}

func (c *chunkedBodyWriter) Flush() error {
	return c.w.Flush()
}

// Finalize will write the ending chunk and flush the writer
func (c *chunkedBodyWriter) Finalize() error {
	err := ext.WriteChunk(c.w, nil)
	if err != nil {
		return err
	}
	return ext.WriteTrailer(c.r.Header.Trailer(), c.w)
}

func NewChunkedBodyWriter(r *protocol.Response, w network.Writer) network.ExtWriter {
	return &chunkedBodyWriter{
		r: r,
		w: w,
	}
}
