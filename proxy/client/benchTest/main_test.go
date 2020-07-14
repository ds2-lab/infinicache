package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"math/rand"
	"sync"
	"testing"
)

var(
	sizes = []int{1,160,650,1300}
	proxies = []string{"10.4.0.100:6378"}
)

func BenchmarkEcSetSimple(b *testing.B) {
	cli := initClient()
	for _, size := range sizes {
		val := make([]byte, size)
		rand.Read(val)
		b.ResetTimer()
		b.Run(fmt.Sprintf("EcSimpleSet/%d B", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_,_,_ = cli.EcSet(fmt.Sprintf("k-%d-%d", i, size), val)
			}
		})
	}
}

func BenchmarkEcGetSimple(b *testing.B){
	cli := initClient()
	// we first set some data to be sure our GET ops are successfull
	allSets := make(map[int][]string)
	for _, size := range sizes {
		val := make([]byte, size)
		rand.Read(val)
		var okSets []string
		for i := 0; i <= 500; i++{
			key := fmt.Sprintf("k-%d-%d", size, i)
			_, _, err := cli.EcSet(key, val)
			if err != false {
				okSets = append(okSets, key)
			}
		}
		allSets[size] = okSets
	}

	for _, size := range sizes {
		setOps := allSets[size]
		b.Run(fmt.Sprintf("EcSimpleGet/%d B", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// we randomly choose some data to GET from the previous set ops
				r := rand.Intn(len(setOps)-1)
				_,_,_,_ = cli.EcGet(setOps[r], size)
			}
		})
	}
}

func BenchmarkEcSetMultiple(b *testing.B) {
	cli := initClient()
	for _, size := range sizes {
		val := make([]byte, size)
		rand.Read(val)
		b.ResetTimer()
		b.Run(fmt.Sprintf("EcMultipleSet/9 x %d B = %d B", size, size*9), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for j := 0; j < 9; j++ {
					_, _, _ = cli.EcSet(fmt.Sprintf("k-%d-%d-%d", i, size, j), val)
				}
			}
		})
	}
}

func BenchmarkEcGetMultiple(b *testing.B){
	cli := initClient()
	// we first set some data to be sure our GET ops are successfull
	allSets := make(map[int][]string)
	for _, size := range sizes {
		val := make([]byte, size)
		rand.Read(val)
		var okSets []string
		for i := 0; i <= 500; i++{
			key := fmt.Sprintf("k-%d-%d", size, i)
			_, _, err := cli.EcSet(key, val)
			if err != false {
				okSets = append(okSets, key)
			}
		}
		allSets[size] = okSets
	}
	b.ResetTimer()
	for _, size := range sizes {
		setOps := allSets[size]
		b.Run(fmt.Sprintf("EcMultipleGet/3*%d B = %d B", size, 3*size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// we randomly choose some data to GET from the previous set ops
				r := rand.Intn(len(setOps)-1)
				for j:=0; j<3; j++{
					_,_,_,_ = cli.EcGet(setOps[r], size)
				}
			}
		})
	}
}

func BenchmarkMkSet(b *testing.B) {
	b.StopTimer()
	cli := initClient()
	for _, size := range sizes {
		data := cli.GenerateSetData(size)
		b.Run(fmt.Sprintf("9 x %d B = %d", size, 9*size), func(b *testing.B) {
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				_,_,_ = cli.MkSet(fmt.Sprintf("k-%d-%d", i, size), data)
			}
		})
	}
}

func BenchmarkMkGet(b *testing.B) {
	cli := initClient()
	// we first set some data to be sure our GET ops are successfull
	allSets := make(map[int][][3]client.KVSetGroup)
	var allKeys []string
	for _, size := range sizes {
		dSet := cli.GenerateSetData(size)
		var okSets [][3]client.KVSetGroup
		for i := 0; i <= 500; i++{
			key := fmt.Sprintf("k-%d", i)
			_, _, err := cli.MkSet(key, dSet)
			if err != false {
				okSets = append(okSets, dSet)
				allKeys = append(allKeys, key)
			}
		}
		allSets[size] = okSets
	}
	b.ResetTimer()
	for _, size := range sizes {
		setOps := allSets[size]
		b.Run(fmt.Sprintf("3 x %d B = %d B ", size, 3*size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// we randomly choose some data to GET from the previous set ops
				r := rand.Intn(len(setOps)-1)
				dGet := cli.GenerateSingleRandomGet(setOps[r])
				b.StartTimer()
				_,_,_ = cli.MkGet(allKeys[r], dGet)
			}
		})
	}
}

func BenchmarkRSet(b *testing.B) {
	b.StopTimer()
	for _, size := range sizes {
		cli := initClient()
		val := make([]byte, size)
		rand.Read(val)
		b.Run(fmt.Sprintf("%d B", size), func(b *testing.B) {
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				_,_,_ = cli.EcSet(fmt.Sprintf("k-%d-%d", i, size), val)
			}
		})
	}
}

func BenchmarkRGet(b *testing.B){
	// we first set some data to be sure our GET ops are successfull
	allSets := make(map[int][]string)
	for _, size := range sizes {
		cli := initClient()
		val := make([]byte, size)
		rand.Read(val)
		var okSets []string
		for i := 0; i <= 500; i++{
			key := fmt.Sprintf("k-%d-%d", size, i)
			_, _, err := cli.RSet(key, val)
			if err != false {
				okSets = append(okSets, key)
			}
		}
		allSets[size] = okSets
	}
	b.ResetTimer()
	for _, size := range sizes {
		cli := initClient()
		setOps := allSets[size]
		b.Run(fmt.Sprintf("%d B", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// we randomly choose some data to GET from the previous set ops
				r := rand.Intn(len(setOps)-1)
				b.StartTimer()
				_,_,_,_ = cli.RGet(setOps[r], size)
			}
		})
	}
}

func BenchmarkParallelRSet(b *testing.B){

	val := make([]byte, 1300)
	rand.Read(val)

	var wg sync.WaitGroup
	for i:=0; i<3; i++ {
		j := i
		wg.Add(1)
		go func(){
			defer wg.Done()
			b.Run(fmt.Sprintf("cli %d - 1300 B", j), func(b *testing.B) {
				b.StopTimer()
				cli := initClient()
				b.StartTimer()
				for k := 0; k < b.N; k++ {
					_,_,_ = cli.EcSet(fmt.Sprintf("k-%d-%d", k, 1300), val)
				}
			})
		}()
		wg.Wait()
	}


}

func initClient() *client.Client {
	cli := client.NewClient(10, 2, 32, 3)
	cli.Dial(proxies)
	return cli
}

