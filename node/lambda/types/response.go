package types

import (
	"bytes"
	"github.com/mason-leap-lab/redeo/resp"
)

type Response struct {
	resp.ResponseWriter

	Cmd string
	ConnId string
	ReqId string
	ChunkId string
	Val string
	Body []byte
	BodyStream resp.AllReadCloser

	LowLevelKeyValuePairs map[string][]byte
}

func (r *Response) Prepare() {
	r.AppendBulkString(r.Cmd)
	r.AppendBulkString(r.ConnId)
	r.AppendBulkString(r.ReqId)
	r.AppendBulkString(r.ChunkId)

	lowLevelKeyValuePairs := len(r.LowLevelKeyValuePairs)
	if lowLevelKeyValuePairs > 0 {
		r.AppendInt(int64(lowLevelKeyValuePairs))
		for k := range r.LowLevelKeyValuePairs{
			r.AppendBulkString(k)
			r.AppendBulk(r.LowLevelKeyValuePairs[k])
		}
	}

	if len(r.Val) > 0 {
		r.AppendBulkString(r.Val)
	}
}

func (r *Response) PrepareByResponse(reader resp.ResponseReader) (err error) {
	r.Cmd, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	r.ConnId, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	r.ReqId, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	r.ChunkId, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	r.BodyStream, err = reader.StreamBulk()
	if err != nil {
		return
	}

	r.Prepare()
	return
}

func (r *Response) Flush() error {
	if r.Body != nil {
		if err := r.CopyBulk(bytes.NewReader(r.Body), int64(len(r.Body))); err != nil {
			return err
		}
	} else if r.BodyStream != nil {
		if err := r.CopyBulk(r.BodyStream, r.BodyStream.Len()); err != nil {
			return err
		}
	}

	return r.ResponseWriter.Flush()
}
