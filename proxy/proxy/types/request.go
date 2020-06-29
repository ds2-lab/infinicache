package types

import (
	"errors"
	"github.com/mason-leap-lab/redeo/resp"
	"strconv"
	"sync/atomic"
)

type Command interface {
	Retriable() bool
	Flush() error
}

type Request struct {
	Id           Id
	Cmd          string
	Key          string
	Body         []byte
	BodyStream   resp.AllReadCloser
	ChanResponse chan interface{}
	EnableCollector bool

	w                *resp.RequestWriter
	responded        uint32
	streamingStarted bool

	LowLevelKeys   []string
	LowLevelValues [][]byte

}

func (req *Request) Retriable() bool {
	return req.BodyStream == nil || !req.streamingStarted
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

func (req *Request) PrepareForMkSet(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(6 + (2*len(req.LowLevelValues)))
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString(req.Id.ChunkId)
	w.WriteBulkString(req.Key)
	w.WriteBulkString(strconv.Itoa(len(req.LowLevelValues)))
	if req.LowLevelValues != nil {
		for i := 0; i < len(req.LowLevelValues); i++ {
			w.WriteBulkString(req.LowLevelKeys[i])
			w.WriteBulk(req.LowLevelValues[i])
		}

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

func (req *Request) PrepareForMkGet(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(7+len(req.LowLevelKeys))
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString(req.Id.ChunkId)
	w.WriteBulkString("")
	w.WriteBulkString(req.Key)
	w.WriteBulkString(strconv.Itoa(len(req.LowLevelKeys)))
	for _, key := range req.LowLevelKeys{
		w.WriteBulkString(key)
	}
	req.w = w
}

//func (req *Request) PrepareForData(w *resp.RequestWriter) {
//	w.WriteMultiBulkSize(1)
//	w.WriteBulkString(req.Cmd)
//	req.w = w
//}

func (req *Request) PrepareForDel(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(5)
	w.WriteBulkString(req.Cmd)
	w.WriteBulkString(strconv.Itoa(req.Id.ConnId))
	w.WriteBulkString(req.Id.ReqId)
	w.WriteBulkString(req.Id.ChunkId)
	w.WriteBulkString(req.Key)
	req.w = w
}

func (req *Request) Flush() error {
	if req.w == nil {
		return errors.New("Writer for request not set.")
	}
	w := req.w
	req.w = nil

	if err := w.Flush(); err != nil {
		return err
	}

	if req.BodyStream != nil {
		req.streamingStarted = true
		if err := w.CopyBulk(req.BodyStream, req.BodyStream.Len()); err != nil {
			return err
		}
		return w.Flush()
	}

	return nil
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
	if req.ChanResponse != nil {
		req.ChanResponse <- &ProxyResponse{ rsp, req }

		// Release reference so chan can be garbage collected.
		req.ChanResponse = nil
	}

	return true
}
