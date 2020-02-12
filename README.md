# InfiniCache

**InfiniCache** is a first-of-its-kind, cost-effective, object cache that is built atop ephemeral cloud funtions. InfiniCache is 31X cheaper than traditional cloud cache services.

Paper: [InfiniCache: Exploiting Ephemeral Serverless Functions to Build a Cost-Effective Memory Cache](https://www.usenix.org/conference/fast20/presentation/wang-ao)

## Prepare
- ### EC2 Proxy

  Amazon EC2 AMI: ubuntu-xenial-16.04
  Golang: 1.12

  Be sure the port of **6378 - 7380** is avaiable on the proxy and EC2 proxy should be under the same VPC group with Lambda function.

  #### Golang install

  [Amazon AMI](https://hackernoon.com/deploying-a-go-application-on-aws-ec2-76390c09c2c5) or [Ubuntu AMI](https://tecadmin.net/install-go-on-ubuntu/)  

  #### Package install

  ```shell
  sudo apt install awscli
  sudo apt install zip
  ```

  ```go
  go get -u github.com/mason-leap-lab/infinicache
  ```

- ### Lambda Runtime

  #### Lambda Role

  Go to AWS IAM console and create a role for the lambda cache node (Lambda function).

  AWS IAM console -> Roles -> Create Role -> Lambda -> `AWSLambdaFullAccess, AWSLambdaVPCAccessExecutionRole, AWSLambdaENIManagementAccess`

  #### Enable Lambda internet access under VPC

  [refer to this article](https://aws.amazon.com/premiumsupport/knowledge-center/internet-access-lambda-function/)

- ### S3

  Create a S3 bucket for storing the zip file of the Lambda code and data output by Lambda functions.

  Run `aws configure` to config your AWS credential.

  ```shell
  aws configure
  ```

- ### Configuration

  #### Lambda function create and config

  Edit deploy/create_function.sh and deploy/update_function.sh

  ```shell
  PREFIX="your lambda function prefix"
  S3="your bucket name"
  cluster=400 # number of lambda in the cache pool
  mem=1536
  ```

  Edit destination S3 in lambda/collector/collector.go, this bucket is for the bill duration log from Cloudwatch

  ```go
  S3BUCKET = "your bucket name"
  ```

  Edit the Lambda execution role and the VPC configuration in deploy/deploy_function.go

  ```go
  ROLE = "your lambda exection role"
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

  Run script to deploy lambda functions

  ```shell
  go get
  deploy/create_function.sh 60
  ```

## Execution

Run `make start` to start proxy server. To stop proxy server, run `make stop`

```
make start
```

### Related repo

Client Library [ecRedis](https://github.com/mason-leap-lab/infinicache/client)  
Redis Protocol [redeo](https://github.com/mason-leap-lab/redeo)  
Benchmark tool [redbench](https://github.com/wangaoone/redbench)  
