# InfiniCache Client Library

Client Library for [infinicache](https://github.com/mason-leap-lab/infinicache)


## Examples
A simple client example with PUT/GET:
```go

package main

import (
	"github.com/mason-leap-lab/infinicache/client"
	"math/rand"
	"strings"
)

var addrList = "127.0.0.1:6378"

func main() {
	// initial object with random value
	var val []byte
	val = make([]byte, 1024)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	cli.EcSet("foo", val)
	cli.EcGet("foo", 1024)
}
```
