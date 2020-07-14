package server

import (
	"time"

	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
)

const LambdaMaxDeployments = 50
const NumLambdaClusters = 50
// fixed size array with lambdas addresses
var LambdaAddresses = [...]string {
	"http://infinicache-node-0.default.svc.cluster.local",
	"http://infinicache-node-1.default.svc.cluster.local",
	"http://infinicache-node-2.default.svc.cluster.local",
	"http://infinicache-node-3.default.svc.cluster.local",
	"http://infinicache-node-4.default.svc.cluster.local",
	"http://infinicache-node-5.default.svc.cluster.local",
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
	"http://infinicache-node-19.default.svc.cluster.local",
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
	"http://infinicache-node-40.default.svc.cluster.local",
	"http://infinicache-node-41.default.svc.cluster.local",
	"http://infinicache-node-42.default.svc.cluster.local",
	"http://infinicache-node-43.default.svc.cluster.local",
	"http://infinicache-node-44.default.svc.cluster.local",
	"http://infinicache-node-45.default.svc.cluster.local",
	"http://infinicache-node-46.default.svc.cluster.local",
	"http://infinicache-node-47.default.svc.cluster.local",
	"http://infinicache-node-48.default.svc.cluster.local",
	"http://infinicache-node-49.default.svc.cluster.local",
/*
	"http://infinicache-node-50.default.svc.cluster.local",
	"http://infinicache-node-51.default.svc.cluster.local",
	"http://infinicache-node-52.default.svc.cluster.local",
	"http://infinicache-node-53.default.svc.cluster.local",
	"http://infinicache-node-54.default.svc.cluster.local",
	"http://infinicache-node-55.default.svc.cluster.local",
	"http://infinicache-node-56.default.svc.cluster.local",
	"http://infinicache-node-57.default.svc.cluster.local",
	"http://infinicache-node-58.default.svc.cluster.local",
	"http://infinicache-node-59.default.svc.cluster.local",
	"http://infinicache-node-60.default.svc.cluster.local",
	"http://infinicache-node-61.default.svc.cluster.local",
	"http://infinicache-node-62.default.svc.cluster.local",
	"http://infinicache-node-63.default.svc.cluster.local",
	"http://infinicache-node-64.default.svc.cluster.local",
	"http://infinicache-node-65.default.svc.cluster.local",
	"http://infinicache-node-66.default.svc.cluster.local",
	"http://infinicache-node-67.default.svc.cluster.local",
	"http://infinicache-node-68.default.svc.cluster.local",
	"http://infinicache-node-69.default.svc.cluster.local",
	"http://infinicache-node-60.default.svc.cluster.local",
	"http://infinicache-node-71.default.svc.cluster.local",
	"http://infinicache-node-72.default.svc.cluster.local",
	"http://infinicache-node-73.default.svc.cluster.local",
	"http://infinicache-node-74.default.svc.cluster.local",
	"http://infinicache-node-75.default.svc.cluster.local",
	"http://infinicache-node-76.default.svc.cluster.local",
	"http://infinicache-node-77.default.svc.cluster.local",
	"http://infinicache-node-78.default.svc.cluster.local",
	"http://infinicache-node-79.default.svc.cluster.local",
	"http://infinicache-node-80.default.svc.cluster.local",
	"http://infinicache-node-81.default.svc.cluster.local",
	"http://infinicache-node-82.default.svc.cluster.local",
	"http://infinicache-node-83.default.svc.cluster.local",
	"http://infinicache-node-84.default.svc.cluster.local",
	"http://infinicache-node-85.default.svc.cluster.local",
	"http://infinicache-node-86.default.svc.cluster.local",
	"http://infinicache-node-87.default.svc.cluster.local",
	"http://infinicache-node-88.default.svc.cluster.local",
	"http://infinicache-node-89.default.svc.cluster.local",
	"http://infinicache-node-80.default.svc.cluster.local",
	"http://infinicache-node-91.default.svc.cluster.local",
	"http://infinicache-node-92.default.svc.cluster.local",
	"http://infinicache-node-93.default.svc.cluster.local",
	"http://infinicache-node-94.default.svc.cluster.local",
	"http://infinicache-node-95.default.svc.cluster.local",
	"http://infinicache-node-96.default.svc.cluster.local",
	"http://infinicache-node-97.default.svc.cluster.local",
	"http://infinicache-node-98.default.svc.cluster.local",
	"http://infinicache-node-99.default.svc.cluster.local",
*/
}
const LambdaStoreName = "LambdaStore" // replica version (no use)
const LambdaPrefix = "CacheNode"
const InstanceWarmTimout = 1 * time.Minute
const InstanceCapacity = 1536 * 1000000 // MB
const InstanceOverhead = 100 * 1000000  // MB
const ServerPublicIp = "10.4.0.100" // Leave it empty if using VPC.
//const ServerPublicIp = "10.4.14.71"

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
