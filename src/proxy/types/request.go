package types

import (
	"errors"
	"github.com/wangaoone/redeo/resp"
	"strconv"
	"sync/atomic"
)

type Request struct {
	Id           Id
	Cmd          string
	Key          string
	Body         []byte
	BodyStream   resp.AllReadCloser
	ChanResponse chan interface{}

	w         *resp.RequestWriter
	responded uint32
}

func (req *Request) PrepareForSet(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(6)
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString(req.Id.ChunkId)
	w.WriteBulkString(req.Key)
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
	w.WriteBulkString(req.Key)
	req.w = w
}

//func (req *Request) PrepareForData(w *resp.RequestWriter) {
//	w.WriteMultiBulkSize(1)
//	w.WriteBulkString(req.Cmd)
//	req.w = w
//}

func (req *Request) PrepareForDel(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(4)
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString(req.Key)
	req.w = w
}

func (req *Request) Flush() (err error) {
	if req.w == nil {
		return errors.New("Writer for request not set.")
	}
	w := req.w
	req.w = nil

	if req.BodyStream != nil {
		if err := w.CopyBulk(req.BodyStream, req.BodyStream.Len()); err != nil {
			return err
		}
	}

	return w.Flush()
}

func (req *Request) IsResponse(rsp *Response) bool {
	return req.Cmd == rsp.Cmd &&
		req.Id.ReqId == rsp.Id.ReqId &&
		req.Id.ChunkId == rsp.Id.ChunkId
}

func (req *Request) SetResponse(rsp interface{}) bool {
	if !atomic.CompareAndSwapUint32(&req.responded, 0, 1) {
		return false
	}

	req.ChanResponse <- rsp

	// Release reference so chan can be garbage collected.
	req.ChanResponse = nil

	return true
}
