package server

import (
	"time"

	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
)

const AWSRegion = "us-east-2"
const LambdaMaxDeployments = 20
const NumLambdaClusters = 20
// fixed size array with lambdas addresses
var LambdaAddresses = [...]string {
	"http://infinicache-node-19.default.example.com",
	"http://infinicache-node-18.default.example.com",
	"http://infinicache-node-17.default.example.com",
	"http://infinicache-node-16.default.example.com",
	"http://infinicache-node-15.default.example.com",
	"http://infinicache-node-14.default.example.com",
	"http://infinicache-node-13.default.example.com",
	"http://infinicache-node-12.default.example.com",
	"http://infinicache-node-10.default.example.com",
	"http://infinicache-node-9.default.example.com",
	"http://infinicache-node-8.default.example.com",
	"http://infinicache-node-7.default.example.com",
	"http://infinicache-node-6.default.example.com",
	"http://infinicache-node-5.default.example.com",
	"http://infinicache-node-4.default.example.com",
	"http://infinicache-node-3.default.example.com",
	"http://infinicache-node-2.default.example.com",
	"http://infinicache-node-1.default.example.com",
	"http://infinicache-node-0.default.example.com",
}
const LambdaStoreName = "LambdaStore" // replica version (no use)
const LambdaPrefix = "CacheNode"
const InstanceWarmTimout = 1 * time.Minute
const InstanceCapacity = 1536 * 1000000 // MB
const InstanceOverhead = 100 * 1000000  // MB
const ServerPublicIp = "" // Leave it empty if using VPC.

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
