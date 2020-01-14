package proxy

import (
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const LambdaMaxDeployments = 64
const NumLambdaClusters = 32
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy1Node"
const InstanceWarmTimout = 1 * time.Minute

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
