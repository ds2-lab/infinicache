package proxy

import (
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const LambdaMaxDeployments = 400
const NumLambdaClusters = 400
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy1Node"
const InstanceWarmTimout = 10 * time.Minute

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
