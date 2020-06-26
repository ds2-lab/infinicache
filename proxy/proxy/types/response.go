package types

import (
	"errors"
	"github.com/mason-leap-lab/redeo/resp"
)

type ProxyResponse struct {
	Response interface{}
	Request *Request
}

type Response struct {
	Id   Id
	Cmd  string
	Body []byte
	BodyStream resp.AllReadCloser

	w    resp.ResponseWriter

	LowLevelKeyValuePairs map[string][]byte
}

func (rsp *Response) PrepareFor(w resp.ResponseWriter) {
	w.AppendBulkString(rsp.Id.ReqId)
	if rsp.Body == nil && rsp.BodyStream == nil {
		w.AppendBulkString("-1")
	} else {
		w.AppendBulkString(rsp.Id.ChunkId)
	}
	if rsp.Body != nil {
		w.AppendBulk(rsp.Body)
	}
	rsp.w = w
}

func (rsp *Response) Flush() error {
	if rsp.w == nil {
		return errors.New("Writer for response not set.")
	}
	w := rsp.w
	rsp.w = nil

	if rsp.BodyStream != nil {
		if err := w.CopyBulk(rsp.BodyStream, rsp.BodyStream.Len()); err != nil {
			return err
		}
	}

	return w.Flush()
}
