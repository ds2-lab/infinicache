package main

import (
	"github.com/neboduus/infinicache/proxy/client"
	"testing"
)


func BenchmarkMerge(b *testing.B) {
	cli := client.NewClient(10, 2, 32, 3)
	merges := []struct {
		name string
		fun  func(key string, val []byte, args ...interface{}) (string, float64, bool)
	}{
		{"MK_SET", cli.EcSet},
	}
	data := cli.GenerateSetData(160)
	for _, merge := range merges {
		b.Run(merge.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_,_,_ = cli.MkSet("test_benchmark", data)
			}
		})
	}
}