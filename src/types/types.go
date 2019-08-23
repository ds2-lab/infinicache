package types

// Input
type InputEvent struct {
	Id           uint64 `json:"id"`
	Timeout      int    `json:"timeout"`
	MigratorAddr string `json:"addr"`
	Cmd          string `json:"cmd"`
}
