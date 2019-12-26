# Lambda Storage
### Prepare
#### Go install (Amazon AMI)
[Amazon AMI](https://hackernoon.com/deploying-a-go-application-on-aws-ec2-76390c09c2c5)  
[Ubuntu AMI](https://tecadmin.net/install-go-on-ubuntu/)  
#### Lambda Role
Go to AWS IAM console and create a basic lambda exection role for the lambda function  
AWS IAM console -> Create role -> Lambda -> AWSLambdaBasicExecutionRole
#### Package
```golang
go get -u github.com/aws/aws-sdk-go/...
go get -u github.com/wangaoone/redeo
go get -u github.com/wangaoone/redeo/resp
go get -u github.com/wangaoone/s3gof3r
go get -u github.com/buraksezer/consistent
go get -u github.com/cespare/xxhash
go get -u github.com/seiflotfy/cuckoofilter
```
#### Related repo
[ecRedis](https://github.com/wangaoone/ecRedis)  
[redeo](https://github.com/wangaoone/redeo)  
[redbench](https://github.com/tddg/redbench)

### Proxy Port
Client facing port： 6379  
Lambda facing port: 6380

### Todos

* Minimize incremental backup cost, the minimize cost is equal to a warmup cost.
* On backup promote to serve, backup itself immiediately.
* Because of the warmup, we may discover lambda failure earlier, and recover data from inexpensive storage without compromising request latency if detected.
* Add clock LRU: https://cs.gmu.edu/~yuecheng/teaching/__cs471_spring19/_static/lecs/lec-04d-mem.pdf
