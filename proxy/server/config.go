package server

import (
	"time"

	"github.com/mason-leap-lab/infinicache/proxy/lambdastore"
)

const LambdaMaxDeployments = 64
const NumLambdaClusters = 32
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy1Node"
const InstanceWarmTimout = 1 * time.Minute
const InstanceCapacity = 1536 * 1000000    // MB
const InstanceOverhead = 100 * 1000000     // MB

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
