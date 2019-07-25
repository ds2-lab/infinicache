# Lambda Storage
### Prepare
#### Go install (Amazon AMI)
[Amazon AMI](https://hackernoon.com/deploying-a-go-application-on-aws-ec2-76390c09c2c5)  
[Ubuntu AMI](https://tecadmin.net/install-go-on-ubuntu/)
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
