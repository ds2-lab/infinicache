package server

import (
	"time"

	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
)

const AWSRegion = "us-east-2"
const LambdaMaxDeployments = 20
const NumLambdaClusters = 20
const LambdaStoreName = "LambdaStore" // replica version (no use)
const LambdaPrefix = "CacheNode"
const InstanceWarmTimout = 1 * time.Minute
const InstanceCapacity = 1536 * 1000000 // MB
const InstanceOverhead = 100 * 1000000  // MB
const ServerPublicIp = "35.204.109.185" // Leave it empty if using VPC.

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
