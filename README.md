# Lambda Storage
### Prepare
#### Go install (Amazon AMI)
[Install Go on Amazon AMI](https://hackernoon.com/deploying-a-go-application-on-aws-ec2-76390c09c2c5)
#### package
```golang
go get -u github.com/wangaoone/redeo
go get -u github.com/wangaoone/redeo/resp
go get -u github.com/aws/aws-sdk-go/...
go get -u github.com/wangaoone/s3gof3r
go get -u github.com/buraksezer/consistent
go get -u github.com/cespare/xxhash
go get -u github.com/seiflotfy/cuckoofilter
```
### Port
Client facing port： 6379  
Lambda facing port: 6380