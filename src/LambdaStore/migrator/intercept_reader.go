package migrator

import (
	"github.com/wangaoone/redeo/resp"
)

type InterceptReader struct {
	resp.AllReadCloser

	buf []byte
	r   int64
}

func NewInterceptReader(reader resp.AllReadCloser) *InterceptReader {
	return &InterceptReader{
		AllReadCloser: reader,
		buf: make([]byte, reader.Len()),
	}
}

func (ir *InterceptReader) Read(p []byte) (n int, err error) {
	n, err = ir.AllReadCloser.Read(p)
	if n > 0 {
		copy(ir.buf[ir.r:], p[0:n])
		ir.r += int64(n)
	}
	return
}

func (ir *InterceptReader) Intercepted() []byte {
	return ir.buf
}
