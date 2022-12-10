package resp

import (
	"bytes"
	"io"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
)

var responseStreamPool = sync.Pool{
	New: func() interface{} {
		return &responseStream{}
	},
}

type responseStream struct {
	prefetchedBytes *bytes.Reader
	header          *protocol.ResponseHeader
	reader          network.Reader
	offset          int
	contentLength   int
	chunkLeft       int
}

func (rs *responseStream) Read(p []byte) (int, error) {
	defer func() {
		if rs.reader != nil {
			rs.reader.Release() //nolint:errcheck
		}
	}()
	if rs.contentLength == -1 {
		if rs.chunkLeft == 0 {
			chunkSize, err := utils.ParseChunkSize(rs.reader)
			if err != nil {
				return 0, err
			}
			if chunkSize == 0 {
				err = ReadTrailer(rs.header, rs.reader)
				if err != nil && err != io.EOF {
					return 0, err
				}
				return 0, io.EOF
			}

			rs.chunkLeft = chunkSize
		}
		bytesToRead := len(p)

		if bytesToRead > rs.chunkLeft {
			bytesToRead = rs.chunkLeft
		}

		src, err := rs.reader.Peek(bytesToRead)
		copied := copy(p, src)
		rs.reader.Skip(copied) // nolint: errcheck
		rs.chunkLeft -= copied

		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return copied, err
		}

		if rs.chunkLeft == 0 {
			err = utils.SkipCRLF(rs.reader)
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
		}
		return copied, err
	}

	if rs.offset == rs.contentLength {
		return 0, io.EOF
	}
	var n int
	var err error
	// read from the pre-read buffer
	if int(rs.prefetchedBytes.Size()) > rs.offset {
		n, err = rs.prefetchedBytes.Read(p)
		rs.offset += n
		if rs.offset == rs.header.ContentLength() {
			return n, io.EOF
		}
		if err != nil || len(p) == n {
			return n, err
		}
	}

	// read from the wire
	m := len(p) - n
	remain := rs.contentLength - rs.offset

	if m > remain {
		m = remain
	}

	if conn, ok := rs.reader.(io.Reader); ok {
		m, err = conn.Read(p[n:])
	} else {
		var tmp []byte
		tmp, err = rs.reader.Peek(m)
		m = copy(p[n:], tmp)
		rs.reader.Skip(m) // nolint: errcheck
	}
	rs.offset += m
	n += m

	if err != nil {
		// the data on stream may be incomplete
		if err == io.EOF {
			if rs.offset != rs.contentLength {
				err = io.ErrUnexpectedEOF
			}
			// ensure that skipRest works fine
			rs.offset = rs.contentLength
		}
		return n, err
	}
	if rs.offset == rs.contentLength {
		err = io.EOF
	}
	return n, err
}

func (rs *responseStream) skipRest() error {
	// The body length doesn't exceed the maxContentLengthInStream or
	// the bodyStream haa been skip rest
	if rs.prefetchedBytes == nil {
		return nil
	}
	// max value of pSize is 8193, it's safe.
	pSize := int(rs.prefetchedBytes.Size())
	if rs.contentLength <= pSize || rs.offset == rs.contentLength {
		return nil
	}

	needSkipLen := 0
	if rs.offset > pSize {
		needSkipLen = rs.contentLength - rs.offset
	} else {
		needSkipLen = rs.contentLength - pSize
	}

	// must skip size
	for {
		skip := rs.reader.Len()
		if skip == 0 {
			_, err := rs.reader.Peek(1)
			if err != nil {
				return err
			}
			skip = rs.reader.Len()
		}
		if skip > needSkipLen {
			skip = needSkipLen
		}
		rs.reader.Skip(skip)
		needSkipLen -= skip
		if needSkipLen == 0 {
			return nil
		}
	}
}

func AcquireResponseStream(b *bytebufferpool.ByteBuffer, r network.Reader, contentLength int, h *protocol.ResponseHeader) io.Reader {
	rs := responseStreamPool.Get().(*responseStream)
	rs.prefetchedBytes = bytes.NewReader(b.B)
	rs.contentLength = contentLength
	rs.reader = r
	rs.header = h
	return rs
}

// ReleaseBodyStream releases the body stream
//
// NOTE: Be careful to use this method unless you know what it's for
func ReleaseResponseStream(requestReader io.Reader) (err error) {
	if rs, ok := requestReader.(*responseStream); ok {
		err = rs.skipRest()
		rs.prefetchedBytes = nil
		rs.offset = 0
		rs.reader = nil
		rs.header = nil
		responseStreamPool.Put(rs)
	}
	return
}
