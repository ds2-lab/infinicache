package types

import (
	"errors"
	"github.com/wangaoone/redeo/resp"
	"strconv"
)

type Request struct {
	Id   Id
	Cmd  string
	Key  []byte
	Body []byte
	BodyStream resp.AllReadCloser
	ChanResponse chan interface{}

	w    *resp.RequestWriter
}

func (req *Request) PrepareForSet(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(6)
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString(req.Id.ChunkId)
	w.WriteBulk(req.Key)
	if req.Body != nil {
		w.WriteBulk(req.Body)
	}
	req.w = w
}

func (req *Request) PrepareForGet(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(5)
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString("")
	w.WriteBulk(req.Key)
	req.w = w
}

func (req *Request) PrepareForData(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(1)
	w.WriteBulkString(req.Cmd)
	req.w = w
}

func (req *Request) Flush() error {
	if req.w == nil {
		return errors.New("Writer for request not set.")
	}
	w := req.w
	req.w = nil

	if req.BodyStream == nil {
		return w.Flush()
	} else {
		return w.CopyBulk(req.BodyStream, req.BodyStream.N())
	}
}

func (req *Request) IsResponse(rsp *Response) bool {
	return req.Cmd == rsp.Cmd &&
		req.Id.ReqId == rsp.Id.ReqId &&
		req.Id.ChunkId == rsp.Id.ChunkId
}
