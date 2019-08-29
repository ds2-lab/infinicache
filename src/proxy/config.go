package proxy

import (
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const LambdaMaxDeployments = 1000
const NumLambdaClusters = 64
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy1Node"
const InstanceWarmTimout = 10 * time.Minute

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
