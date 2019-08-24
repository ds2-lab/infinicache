package types

// Input
type InputEvent struct {
	Cmd          string `json:"cmd"`
	Id           uint64 `json:"id"`
	Timeout      int    `json:"timeout"`
	Addr         string `json:"addr"`
}
