package main

import (
	"fmt"
	"strconv"
)

var LambdaAddresses = [...]string {
	/*	"http://infinicache-node-0.default.34.91.116.154.xip.io",
		"http://infinicache-node-1.default.34.91.116.154.xip.io",
		"http://infinicache-node-2.default.34.91.116.154.xip.io",
		"http://infinicache-node-3.default.34.91.116.154.xip.io",
		"http://infinicache-node-4.default.34.91.116.154.xip.io",
		"http://infinicache-node-5.default.34.91.116.154.xip.io",
		"http://infinicache-node-6.default.34.91.116.154.xip.io",
		"http://infinicache-node-7.default.34.91.116.154.xip.io",
		"http://infinicache-node-8.default.34.91.116.154.xip.io",
		"http://infinicache-node-9.default.34.91.116.154.xip.io",
		"http://infinicache-node-10.default.34.91.116.154.xip.io",
		"http://infinicache-node-12.default.34.91.116.154.xip.io",
		"http://infinicache-node-13.default.34.91.116.154.xip.io",
		"http://infinicache-node-14.default.34.91.116.154.xip.io",
		"http://infinicache-node-15.default.34.91.116.154.xip.io",
		"http://infinicache-node-16.default.34.91.116.154.xip.io",
		"http://infinicache-node-17.default.34.91.116.154.xip.io",
		"http://infinicache-node-18.default.34.91.116.154.xip.io",
		"http://infinicache-node-19.default.34.91.116.154.xip.io",*/
	/*	"http://infinicache-node-0.default.svc.cluster.local",
		"http://infinicache-node-1.default.svc.cluster.local",
		"http://infinicache-node-2.default.svc.cluster.local",
		"http://infinicache-node-3.default.svc.cluster.local",
		"http://infinicache-node-4.default.svc.cluster.local",		"http://infinicache-node-5.default.svc.cluster.local",
		"http://infinicache-node-6.default.svc.cluster.local",
		"http://infinicache-node-7.default.svc.cluster.local",
		"http://infinicache-node-8.default.svc.cluster.local",
		"http://infinicache-node-9.default.svc.cluster.local",
		"http://infinicache-node-10.default.svc.cluster.local",
		"http://infinicache-node-11.default.svc.cluster.local",
		"http://infinicache-node-12.default.svc.cluster.local",
		"http://infinicache-node-13.default.svc.cluster.local",
		"http://infinicache-node-14.default.svc.cluster.local",
		"http://infinicache-node-15.default.svc.cluster.local",
		"http://infinicache-node-16.default.svc.cluster.local",
		"http://infinicache-node-17.default.svc.cluster.local",
		"http://infinicache-node-18.default.svc.cluster.local",
		"http://infinicache-node-19.default.svc.cluster.local",*/
	"http://infinicache-node-20.default.svc.cluster.local",
	"http://infinicache-node-21.default.svc.cluster.local",
	"http://infinicache-node-22.default.svc.cluster.local",
	"http://infinicache-node-23.default.svc.cluster.local",
	"http://infinicache-node-24.default.svc.cluster.local",
	"http://infinicache-node-25.default.svc.cluster.local",
	"http://infinicache-node-26.default.svc.cluster.local",
	"http://infinicache-node-27.default.svc.cluster.local",
	"http://infinicache-node-28.default.svc.cluster.local",
	"http://infinicache-node-29.default.svc.cluster.local",
	"http://infinicache-node-30.default.svc.cluster.local",
	"http://infinicache-node-31.default.svc.cluster.local",
	"http://infinicache-node-32.default.svc.cluster.local",
	"http://infinicache-node-33.default.svc.cluster.local",
	"http://infinicache-node-34.default.svc.cluster.local",
	"http://infinicache-node-35.default.svc.cluster.local",
	"http://infinicache-node-36.default.svc.cluster.local",
	"http://infinicache-node-37.default.svc.cluster.local",
	"http://infinicache-node-38.default.svc.cluster.local",
	"http://infinicache-node-39.default.svc.cluster.local",
}

func main() {
	fmt.Println(len(LambdaAddresses))

	for _, address := range LambdaAddresses {
		var a string
		var b int
		for j, char := range address{
			if j<24{continue}
			if j==24{
				a = string(char)
			}
			if j==25{
				c, e := strconv.Atoi(string(char))
				b = c
				if e==nil {
					a = fmt.Sprintf("%s%d", a, b)
				}
				break
			}
		}
		i, _ := strconv.Atoi(a)
		fmt.Println(i, address)
	}


}
