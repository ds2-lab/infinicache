package types

// Input
type InputEvent struct {
	Cmd     string `json:"cmd"`
	Id      uint64 `json:"id,string"`
	Proxy   string `json:"proxy"`
	Timeout int    `json:"timeout,string"`
	Addr    string `json:"addr"`
	Prefix  string `json:"prefix"`
	Log     int    `json:"log,string"`
}
