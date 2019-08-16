package types

import (
	"bytes"
	"github.com/wangaoone/redeo/resp"
)

type Response struct {
	resp.ResponseWriter

	Cmd string
	ConnId string
	ReqId string
	ChunkId string
	Val string
	Body []byte
}

func (r *Response) Prepare() {
	r.AppendBulkString(r.Cmd)
	r.AppendBulkString(r.ConnId)
	r.AppendBulkString(r.ReqId)
	r.AppendBulkString(r.ChunkId)
	if len(r.Val) > 0 {
		r.AppendBulkString(r.Val)
	}
}

func (r *Response) Flush() error {
	if r.Body == nil {
		return r.ResponseWriter.Flush()
	} else {
		return r.CopyBulk(bytes.NewReader(r.Body), int64(len(r.Body)))
	}
}
