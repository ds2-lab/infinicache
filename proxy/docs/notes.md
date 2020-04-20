### Infinicache
Infinicache is a distributed system build over and for AWS Lambda framework which implements a in-memory cache capable of exploiting serverless advantages. It does so by deploying several lambda nodes which store in memory chunks of files which are managed by a proxy and a client. Current implementation available [here](https://github.com/mason-leap-lab/infinicache) and original paper [here](https://www.usenix.org/conference/fast20/presentation/wang-ao).

* **Client**: Exposes to the application a clean set of GET(key) and PUT(key, value) APIs. The client library is responsible for: (1) transparently handling object encoding/decoding using an embedded EC module, (2) load balancing the requests across a distributed set of proxies, and (3) determining where EC-encoded chunks are placed on a cluster of Lambda nodes.

* **Proxy**: Responsible for: (1) managing a pool of Lambda nodes, and (2) streaming data between clients and the Lambda nodes. Each Lambda node proactively establishes a persistent TCP connection with its managing proxy.

* **Lambda Function Runtime**: Executes inside each Lambda instance and is designed to manage the cached object chunks in the function’s memory

For a deeper understanding about each component and their overall protocol, please refer to the original paper [InfiniCache: Exploiting Ephemeral Serverless Functions to Build a Cost-Effective Memory Cache](https://www.usenix.org/conference/fast20/presentation/wang-ao)

### Implementation Description 
The [implementation](https://github.com/mason-leap-lab/infinicache) is written in Go and is composed by a `module github.com/mason-leap-lab/infinicache` which contains 10 `packages`. The project `go.mod` file shows that it uses 13 external libraries, of which 5 indirect.

The directly imported libraries are:
* [github.com/ScottMansfield/nanolog v0.2.0](https://github.com/ScottMansfield/nanolog) - nanosecond scale logger
*	[github.com/aws/aws-lambda-go v1.13.3](https://github.com/aws/aws-lambda-go) - tools to help Go developers develop AWS Lambda functions
*	[github.com/aws/aws-sdk-go v1.28.10](https://github.com/aws/aws-sdk-go) - AWS SDK for the Go programming language
*	[github.com/buraksezer/consistent v0.0.0-20191006190839-693edf70fd72](https://github.com/buraksezer/consistent) - provides a consistent hashing function
*	[github.com/cespare/xxhash v1.1.0](https://github.com/cespare/xxhash) -  Go implementation of the 64-bit [xxHash](http://cyan4973.github.io/xxHash/) algorithm
*	[github.com/cornelk/hashmap v1.0.1](https://github.com/cornelk/hashmap) - lock-free thread-safe HashMap
*	[github.com/google/uuid v1.1.1](https://github.com/google/uuid) - generates and inspects UUIDs
*	[github.com/klauspost/reedsolomon v1.9.3](https://github.com/klauspost/reedsolomon) - Reed-Solomon Erasure Coding in Go
*	[github.com/mason-leap-lab/redeo v0.0.0-20200204234106-1e6f10c82f05](https://github.com/mason-leap-lab/redeo) -  for building redis-protocol compatible servers/services
*	[github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b](https://github.com/mgutz/ansi) - allows to create ANSI colored strings and codes
*	[github.com/onsi/ginkgo v1.7.0](https://github.com/onsi/ginkgo) - testing
*	[github.com/onsi/gomega v1.4.3](https://github.com/onsi/gomega) - matcher lib for ginko 
*	[github.com/seiflotfy/cuckoofilter v0.0.0-20200106165036-28deee3eabd7](https://github.com/seiflotfy/cuckoofilter) - bloom filter that efficiently allows also deletion

The packages which compose this implementation of infinicache are:
* `global` - contains procedures for global configuration (set ip address, ports, etc..) and is composed by the following files:
  * `infinicache-master/proxy/global/global.go`
  * `infinicache-master/proxy/global/ip.go`
    The global package does not depend on AWS
* `client` contains the client protocol and is composed by the following files:
  * `infinicache-master/client/client.go`
  * `infinicache-master/client/ec.go`
  * `infinicache-master/client/ecRedis.go`
  * `infinicache-master/client/log.go`
    Moreover the client implementation:
    * Does not depend of AWS libraries. 
    * But uses external libraries 
      * [github.com/mason-leap-lab/redeo/resp](github.com/mason-leap-lab/redeo/resp) to implement RESP standard
      * [github.com/buraksezer/consistent](github.com/buraksezer/consistent) for consistent hashing, and
      * [github.com/seiflotfy/cuckoofilter](github.com/seiflotfy/cuckoofilter) to implement a bloom filter that efficiently allows also deletion 
      
* `server` implements the proxy server and contains files:
  * `infinicache-master/proxy/server/config.go` - Lambda runtime config
  * `infinicache-master/proxy/server/group.go` - Group - manages GroupInstances of LambdaDeployments
  * `infinicache-master/proxy/server/meta.go` - represents an object to be stored in cache (called `meta`)
  * `infinicache-master/proxy/server/metastore.go` - a store of metas
  * `infinicache-master/proxy/server/placer.go` - implements a Clock LRU for object/metas removal
  * `infinicache-master/proxy/server/proxy.go` - the main file of the Proxy server implementation which initializes the lambda group and manages client operations. It uses other local packages to manage things such as `infinicache/proxy/collector`, `infinicache/proxy/lambdastore` 
  * `infinicache-master/proxy/server/scheduler.go` - scheduler of the requests (I think, I have to !!!analyze better!!! how it is used and for what)
  
* `main` contains files for deploying a lambda function in AWS, the handler for the function and the proxy main file:
  * `infinicache-master/deploy/deploy_function.go` - deploys the function (Lambda) on AWS, hence uses `github.com/aws/aws-sdk-go`
  * `infinicache-master/lambda/handler.go` - handler file for Lamda Runtime, it is written for AWS Lambda runtime, hence imports AWS SDK, in particular `github.com/aws/aws-lambda-go/lambda and` and `github.com/aws/aws-lambda-go/lambdacontext`. Basically it handles the commands provenient from proxy.
  * `infinicache-master/proxy/proxy.go` - main handler file for proxy, it lunches/initializes the proxy server and all the components that it uses
  
* `collector` (I have to investigate it better because it is not well understood why the `lambda/collector` is accessing AWS S3). The package contais the following files:
  * `infinicache-master/lambda/collector/collector.go` - updates DataEntries to S3, probably it saves data on the permanent storage if data is new?? ToDo: !!!analyze better!!! (uses `github.com/aws/aws-sdk-go/`)
  * `infinicache-master/proxy/collector/collector.go` - collects data from Lambdas?? ToDo: !!!analyze better!!! 
  
* `storage` used by lambda runtimes to store data chunks. Contains files:
  * `infinicache-master/lambda/storage/storage.go` - chunk storage for cache nodes
  
* `migrator` implements the relay that allow to perform the backup protocol for maximizing data availability. Given that AWS Lambda does not inbound TCP/UDP connections, the authors of infinicache came out with a protocol wich overcomes this disadvantage. As described in the original paper this is highly related to AWS implementation. This package contains the following files:
  * `infinicache-master/lambda/migrator/client.go`
  * `infinicache-master/lambda/migrator/intercept_reader.go`
  * `infinicache-master/lambda/migrator/storage_adapter.go`
  * `infinicache-master/migrator/forward_connection.go`
  * `infinicache-master/migrator/migrator.go`
  
* `lifetime` - Cit. "Each Lambda runtime is warmed up after every T_warm interval of time" the authors use a T_warm value of 1 minute as motivated by observations made in the original paper. This package should manage Functions lifespan.
  * `infinicache-master/lambda/lifetime/lifetime.go`
  * `infinicache-master/lambda/lifetime/session.go`
  * `infinicache-master/lambda/lifetime/timeout.go` - this file uses `github.com/aws/aws-lambda-go/lambdacontext` but just to choose some constants based on the type of instance of lambda in therms of CPU power, hence it can be easily replaced with some other api call that does the same
  
* `lambdastore` - 
  * `infinicache-master/proxy/lambdastore/connection.go`
  * `infinicache-master/proxy/lambdastore/deployment.go`
  * `infinicache-master/proxy/lambdastore/instance.go`
  * `infinicache-master/proxy/lambdastore/meta.go`
  
* `types` contains files:
  * `infinicache-master/common/types/types.go`
  * `infinicache-master/lambda/types/response.go`
  * `infinicache-master/lambda/types/types.go`
  * `infinicache-master/proxy/types/control.go`
  * `infinicache-master/proxy/types/request.go`
  * `infinicache-master/proxy/types/response.go`
  * `infinicache-master/proxy/types/types.go`
Notice that test files (which can be identified under `*_test.go` filename) are omitted from this description.            

### Reimplementation VS Migration Analysis
In order to analyze the feasibility of both reimplementation and migration, the evaluation was abstracted by the following questions:
* Which are the theoretical dependencies of Infinicache from AWS Lambda?
    
    1. **AWS Lambda do not offer TCP/UDP invound connections**
       * Paper cit (1): `Since AWS Lambda does not allow inbound TCP or UDP connections, each Lambda runtime establishes a TCP connection with its designated proxy server, the first time it is invoked. A Lambda node gets its proxy’s connection information via its  invocation parameters. The Lambda runtime then keeps the TCP connection established until reclaimed by the provider`
       * The above observation and infinicache workaround, this represents a high limitation for AWS Lambda, because in order to keep an established connection a lot of management is needed. The proxy has to validate the connection many times because the connection may have been reclaimed by the provider. 
       * Knativ should be able to overcome this limitation. Beeing able to deploy stateless containers in a serverless manner, the cache node function, which in this case will live inside a light container deployed by K8s, should be able to inherit all functionalities that containers have and one of these functionalities is TCP/UDP inbound connections. This means that infinicache gains the "server" part, where a Knative function could run as a short-lived server that can accept and serve inbound connections. 
    2. **Anticipatory Billed Duration Control**
        * Paper cit (2): `AWS charges Lambda usage per 100 ms (which we call a billing cycle). To maximize the use of each billing cycle and to avoid the overhead of restarting Lambdas, INFINICACHE’s Lambda runtime uses a timeout scheme to control how long a Lambda function runs. When a Lambda node is invoked by a chunk request, a timer is triggered to limit the function’s execution time. The timeout is initially set to expire within the first billing cycle. The runtime employs a simple heuristic to decide whether to extend the timeout window. If no further chunk request arrives within the first billing cycle, the timer expires and returns 2–10 ms (a short time buffer) before the 100 ms window ends. This avoids accidentally executing into the next billing cycle. The buffer time is configurable, and is empirically decided based on the Lambda function’s memory capacity. If more than one request can be served within the current billing cycle, the heuristic extends the timeout by one more billing cycle, anticipating more incoming requests.`
        * If Infinicache would be ported over Knative, this option will highly depend on the cloud provider. Assuming that Knative would run over Google Cloud provider, this will highly depend on how K8s is configured to run our serverless workloads and which Google Cloud Products will be used to allow K8s to do that. This is true because Knative could be installed everywhere where K8s is installed, hence it could be also installed in AWS. A deeper analysis on Google Cloud Pricing for running serverless workloads using Knative has to be done. 
        * However it seems that this option is configurable, hence once we know how serverless workload is charged by the cloud provider, this option can be tuned.
    2. **Preflight Message**
    * Paper cit (1): `Lambda functions are hosted by EC2 Virtual Machines (VMs). A single VM can host one or more functions. AWS seems to provision Lambda functions on the smallest possible number of VMs using a greedy binpacking heuristic. This could cause severe network bandwidth contention if multiple network-intensive Lambda functions get allocated on the same host VM`.
    * Paper cit (2): `While over-provisioning a large Lambda node pool with many small Lambda functions would help to statistically reduce the chances of Lambda co-location, we find that using relatively bigger Lambda functions largely eliminates Lambda co-location. Lambda’s VM hosts have approximately 3 GB memory. As such, if we use Lambda functions with ≥ 1.5 GB memory, every VM host is occupied exclusively by a single Lambda function, assuming INFINICACHE’s cache pool consists of Lambda functions with the same configuration. Moreover AWS does not allow sharing Lambda-hosting VMs across tenants`
    * Infinicache takes into consideration the above observations, but with Knative this may not be true. Given that it is build over Kubernetes, it follows its rules in therms of container collocation, hence to better understand this, an analysis about how this is managed in K8s has to be done. !!!analyze new!!!. However this is highly related to the cloud provider which manages the cluster. K8s also could provide a usefull interface to a configuration process of this aspect, but this aspect has to be further investigated.

* How these dependencies are translated in the Go implementation?
* In the prospective of implementing Infinicache over Knative, would these theoretical platform dependencies exist?
* What would change if infinicache would be implemented over Knative?

### Go Programming Language (Red/Learned Chapters)
* Packages, variables, and functions
* Flow control statements: for, if, else, switch and defer
* More types: structs, slices, and maps
* Methods and interfaces
* Concurrency

### Some usefull articles 
* [Functions vs Containers](https://medium.com/oracledevs/containers-vs-functions-51c879216b97)
* [Building functions with Riff - Blog Post](https://tanzu.vmware.com/content/blog/building-functions-with-riff)
* [Knative Enables Portable Serverless Platforms on Kubernetes, for Any Cloud](https://thenewstack.io/knative-enables-portable-serverless-platforms-on-kubernetes-for-any-cloud/)

### AWS

* proxy instance ip: `3.22.8.119`
* SSH to the proxy instance
  ` sudo ssh -i ~/.ssh/FraudioAWS.pem ubuntu@ec2-3-14-65-17.us-east-2.compute.amazonaws.com`
* transfer files to instance
  `scp -i ~/.ssh/FraudioAWS.pem /home/nepotu/Desktop/fraudio/projects/infinicache-master/install_go.md ubuntu@ec2-3-14-65-17.us-east-2.compute.amazonaws.com:`
* transfer from instance to local
  `scp -i /path/my-key-pair.pem ec2-user@ec2-198-51-100-1.compute-1.amazonaws.com:~/SampleFile.txt ~/SampleFile2.txt`
