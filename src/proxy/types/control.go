package types

import (
	"errors"
	"github.com/wangaoone/redeo/resp"
)

type Control struct {
	Cmd  string
	Body string
	w    *resp.RequestWriter
}

func (ctrl *Control) PrepareForData(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(1)
	w.WriteBulkString(ctrl.Cmd)
	ctrl.w = w
}

func (ctrl *Control) PrepareForBackup(w *resp.RequestWriter) {
	w.WriteMultiBulkSize(2)
	w.WriteBulkString(ctrl.Cmd)
	w.WriteBulkString(ctrl.Body)
	ctrl.w = w
}

func (ctrl *Control) Flush() (err error) {
	if ctrl.w == nil {
		return errors.New("Writer for request not set.")
	}
	w := ctrl.w
	ctrl.w = nil

	return w.Flush()
}

//func (ctrl *Control) IsResponse(rsp *Response) bool {
//	return ctrl.Cmd == rsp.Cmd &&
//		ctrl.Id.ReqId == rsp.Id.ReqId &&
//		ctrl.Id.ChunkId == rsp.Id.ChunkId
//}
