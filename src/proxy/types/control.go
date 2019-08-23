package types

import (
	"errors"
	"github.com/wangaoone/redeo/resp"
	"strconv"
)

type Control struct {
	Cmd  string
	Addr string
	Deployment string
	Id   uint64
	w    *resp.RequestWriter
}

func (ctrl *Control) PrepareForData(w *resp.RequestWriter) {
	w.WriteCmdString(ctrl.Cmd)
	ctrl.w = w
}

func (ctrl *Control) PrepareForBackup(w *resp.RequestWriter) {
	w.WriteCmdString(ctrl.Cmd, ctrl.Addr, ctrl.Deployment, strconv.FormatUint(ctrl.Id, 10))
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
