package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"testing"
)

func BenchmarkMkSet(b *testing.B) {
	sizes := []int{1,160,500,1600}
	cli := client.NewClient(10, 2, 32, 3)
	cli.Dial([]string{"10.4.0.100:6378"})
	for size := range sizes {
		data := cli.GenerateSetData(size)
		b.Run(fmt.Sprintf("/%d B", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_,_,_ = cli.MkSet(fmt.Sprintf("k-%d-%d", i, size), data)
			}
		})
	}
}

func BenchmarkMkGet(b *testing.B) {
	sizes := []int{1,160,500,1600}
	cli := client.NewClient(10, 2, 32, 3)
	cli.Dial([]string{"10.4.0.100:6378"})
	for size := range sizes {
		d_set := cli.GenerateSetData(size)
		d_get := cli.GenerateSingleRandomGet(d_set)
		b.Run(fmt.Sprintf("/%d B", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_,_,_ = cli.MkGet(fmt.Sprintf("k-%d-%d", i, size), d_get)
			}
		})
	}

}