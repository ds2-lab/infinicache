package types

import (
	"errors"
	"fmt"
	"github.com/wangaoone/redeo/resp"
	"strconv"
	"time"
)

var (
	ErrNotFound = errors.New("Key not found")
)

type Storage interface {
	Get(string) (string, resp.AllReadCloser, error)
	Set(string, string, []byte)
	Len() int
	Keys()  <-chan string
}

// For storage
type Chunk struct {
	Key      string
	Id       string
	Body     []byte
	Accessed time.Time
}

func NewChunk(id string, body []byte) *Chunk {
	return &Chunk{ Id: id, Body: body, Accessed: time.Now() }
}

func (c *Chunk) Access() []byte {
	c.Accessed = time.Now()
	return c.Body
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
	LambdaReqId    string
}

type ResponseError struct {
	error
	StatusCode int
}

func NewResponseError(status int, msg interface{}, args ...interface{}) *ResponseError {
	switch msg.(type) {
	case error:
		return &ResponseError{
			error: msg.(error),
			StatusCode: status,
		}
	default:
		return &ResponseError{
			error: errors.New(fmt.Sprintf(msg.(string), args...)),
			StatusCode: status,
		}
	}
}

func (e *ResponseError) Status() string {
	return strconv.Itoa(e.StatusCode)
}
