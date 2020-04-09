package types

import (
	"errors"
	"fmt"
	"github.com/mason-leap-lab/redeo/resp"
	"strconv"
	"time"
)

const (
	OP_SET         = 0
	OP_GET         = 1
	OP_DEL		   = 2
	OP_WARMUP      = 90
	OP_MIGRATION   = 91
)

var (
	ErrProxyClosing               = errors.New("Proxy closed.")
	ErrNotFound                   = errors.New("Key not found")
)

type Storage interface {
	Get(string) (string, []byte, error)
	GetStream(string) (string, resp.AllReadCloser, error)
	Set(string, string, []byte) error
	SetStream(string, string, resp.AllReadCloser) error
	Len() int
	Del(string,string) error
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
	Op             int
	Status         string
	ReqId          string
	ChunkId        string
	DurationAppend time.Duration
	DurationFlush  time.Duration
	Duration       time.Duration
	Session        string
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
