package types

import (
	"time"
)

// For storage
type Chunk struct {
	Id   string
	Body []byte
}

// For data collection
type DataEntry struct {
	Op             string
	Status         string
	ReqId          string
	ChunkId        string
	DurationAppend time.Duration
	DurationFlush  time.Duration
	Duration       time.Duration
}
