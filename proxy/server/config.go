package server

import (
	"time"

	"github.com/mason-leap-lab/infinicache/proxy/lambdastore"
)

const LambdaMaxDeployments = 400
const NumLambdaClusters = 400
const LambdaStoreName = "LambdaStore" // replica version (no use)
const LambdaPrefix = "Your Lambda Function Prefix"
const InstanceWarmTimout = 1 * time.Minute
const InstanceCapacity = 1536 * 1000000    // MB
const InstanceOverhead = 100 * 1000000     // MB

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
