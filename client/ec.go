package ecRedis

import (
	"fmt"
	"errors"
	"github.com/klauspost/reedsolomon"
	"io"
)

var (
	ErrNotImplemented = errors.New("Not implemented")
)

func NewEncoder(dataShards int, parityShards int, ecMaxGoroutine int) reedsolomon.Encoder {
	if parityShards == 0 {
		return &DummyEncoder{ DataShards: dataShards }
	}

	enc, err := reedsolomon.New(dataShards, parityShards, reedsolomon.WithMaxGoroutines(ecMaxGoroutine))
	if err != nil {
		fmt.Println("newEncoder err", err)
	}
	return enc
}

type DummyEncoder struct {
	DataShards int
}

func (enc *DummyEncoder) Encode(shards [][]byte) error {
	return nil
}

func (enc *DummyEncoder) Verify(shards [][]byte) (bool, error) {
	if len(shards) != enc.DataShards {
		return false, reedsolomon.ErrTooFewShards
	}

	for _, shard := range shards {
		if len(shard) == 0 {
			return false, reedsolomon.ErrTooFewShards
		}
	}
	return true, nil
}

func (enc *DummyEncoder) Reconstruct(shards [][]byte) (err error) {
	_, err = enc.Verify(shards)
	return
}

func (enc *DummyEncoder) ReconstructData(shards [][]byte) (err error) {
	_, err = enc.Verify(shards)
	return
}

func (enc *DummyEncoder) Update(shards [][]byte, newDatashards [][]byte) error {
	return ErrNotImplemented
}

func (enc *DummyEncoder) Split(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, reedsolomon.ErrShortData
	}
	// Calculate number of bytes per data shard.
	perShard := (len(data) + enc.DataShards - 1) / enc.DataShards

	// Split into shards, the size of shards may be not the same.
	dst := make([][]byte, enc.DataShards)
	i := 0
	for ; i < len(dst) && len(data) >= perShard; i++ {
		dst[i] = data[:perShard]
		data = data[perShard:]
	}

	if i < len(dst) {
		dst[i] = data
	}

	return dst, nil
}

func (enc *DummyEncoder) Join(dst io.Writer, shards [][]byte, outSize int) error {
	// Do we have enough shards?
	if len(shards) < enc.DataShards {
		return reedsolomon.ErrTooFewShards
	}
	shards = shards[:enc.DataShards]

	// Do we have enough data?
	size := 0
	for _, shard := range shards {
		if shard == nil {
			return reedsolomon.ErrReconstructRequired
		}
		size += len(shard)

		// Do we have enough data already?
		if size >= outSize {
			break
		}
	}
	if size < outSize {
		return reedsolomon.ErrShortData
	}

	// Copy data to dst
	write := outSize
	for _, shard := range shards {
		if write < len(shard) {
			_, err := dst.Write(shard[:write])
			return err
		}
		n, err := dst.Write(shard)
		if err != nil {
			return err
		}
		write -= n
	}
	return nil
}
