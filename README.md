# InfiniCache

**InfiniCache** is the first-of-its-kind, cost-effective, and high-performance, in-memory object cache that is built atop ephemeral cloud functions. InfiniCache is 31x - 96x cheaper than traditional cloud cache services.

Our FAST'20 Paper: [InfiniCache: Exploiting Ephemeral Serverless Functions to Build a Cost-Effective Memory Cache](https://www.usenix.org/conference/fast20/presentation/wang-ao)


### Press:
* IEEE Spectrum: [Cloud Services Tool Lets You Pay for Data You Use—Not Data You Store](https://spectrum.ieee.org/tech-talk/computing/networks/pay-cloud-services-data-tool-news)
* Mikhail Shilkov's detailed paper review: [InfiniCache: Distributed Cache on Top of AWS Lambda (paper review)](https://mikhail.io/2020/03/infinicache-distributed-cache-on-aws-lambda/)

## Change log

- **03/07/2020:** Updated deploy procedure and fixed the bug (incorrect path) in the scripts under `deploy/`.

## Prepare

- ### EC2 Proxy

  Amazon EC2 AMI: ubuntu-xenial-16.04

  Golang version: 1.12

  Be sure the port **6378 - 7380** is avaiable on the proxy

  We recommend that EC2 proxy and Lambda functions are under the same VPC network, and deploy InfiniCache on a EC2 instance with powerful CPU and high bandwidth (`c5n` family maybe a good choice).

- ### Golang install

  Jump to [install_go.md](https://github.com/mason-leap-lab/infinicache/blob/master/install_go.md)

- ### Package install

  Install basic package
  ```shell
  sudo apt-get update
  sudo apt-get -y upgrade
  sudo apt install awscli
  sudo apt install zip
  ```

  Clone this repo
  ```go
  go get -u github.com/mason-leap-lab/infinicache
  ```

  Run `aws configure` to setup your AWS credential.

  ```shell
  aws configure
  ```

- ### Lambda Runtime

  #### Lambda Role setup

  Go to AWS IAM console and create a role for the lambda cache node (Lambda function).

  AWS IAM console -> Roles -> Create Role -> Lambda ->

  **`AWSLambdaFullAccess, `**

  **`AWSLambdaVPCAccessExecutionRole, `**

  **`AWSLambdaENIManagementAccess`**

  #### Enable Lambda internet access under VPC

  Plese [refer to this article](https://aws.amazon.com/premiumsupport/knowledge-center/internet-access-lambda-function/). (You could skip this step if you do not want to run InfiniCache under VPC).

- ### S3

  Create the S3 bucket to store the zip file of the Lambda code and data output from Lambda functions. Remember the name of this bucket for the configuration in next step.


- ### Configuration

  #### Lambda function create and config

  Edit `deploy/create_function.sh` and `deploy/update_function.sh`

  ```shell
  PREFIX="your lambda function prefix"
  S3="your bucket name"
  cluster=400 # number of lambda in the cache pool
  mem=1536
  ```

  Edit destination S3 bucket in `lambda/collector/collector.go`, this bucket is for the bill duration log from CloudWatch.
  ```go
  AWSRegion = "us-east-1"
  S3BUCKET = "your bucket name"
  ```

  Edit `lambda/migrator/client.go`,  change AWS region if necessary.
  ```go
  AWSRegion = "us-east-1"
  ```

  Edit the aws settings and the VPC configuration in `deploy/deploy_function.go`. If you do not want to run InfiniCache under VPC, you do not need to modify the `subnet` and `securityGroup` settings.

  ```go
  ROLE = "arn:aws:iam::[aws account id]:role/[role name]"
  REGION = "us-east-1"
  ...
  ...
  subnet = []*string{
    aws.String("your subnet 1"),
    aws.String("your subnet 2"),
  }
  securityGroup = []*string{
    aws.String("your security group")
  }
  ```

  Run script to create and deploy lambda functions (Also, if you do not want to run InfiniCache under VPC, you need to set the `vpc` flag to be `false` in `deploy/create_function.sh`).

  ```shell
  export GO111MODULE="on"
  go get
  deploy/create_function.sh 60
  ```

  #### Proxy configuration

  Edit `proxy/server/config.go`, change the aws region, cluster size, and prefix of the Lambda functions.
  ```go
  const AWSRegion = "us-east-1"
  const NumLambdaClusters = 400
  const LambdaPrefix = "Your Lambda Function Prefix"
  const ServerPublicIp = ""  // Leave it empty if using VPC.
  ```

## Execution

- Proxy server

  Run `make start` to start proxy server.  `make start` would print nothing to the console. If you want to check the log message, you need to set the `debug` flag to be `true` in the `proxy/proxy.go`.

  ```bash
  make start
  ```

  To stop proxy server, run `make stop`. If `make stop` is not working, you could use `pgrep proxy`, `pgrep go` to find the pid, and check the `infinicache pid` and kill them.

- Client library

  The toy demo for Client Library

  ```bash
  go run client/example/main.go
  ```

  The result should be

  ```bash
  ~$ go run client/example/main.go
  2020/03/08 05:05:19 EcRedis Set foo 14630930
  2020/03/08 05:05:19 EcRedis Got foo 3551124 ( 2677371 865495 )
  ```

## Related repo

Client library [ecRedis](https://github.com/mason-leap-lab/infinicache/tree/master/client)  
Redis protocol [redeo](https://github.com/mason-leap-lab/redeo)  
Benchmarking tool [redbench](https://github.com/wangaoone/redbench)  


## Contributing

Please feel free to hack on InfiniCache! We're happy to accept contributions.
