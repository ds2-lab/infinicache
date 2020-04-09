#!/bin/bash

PWD=`dirname $0`
PREFIX="CacheNode"
KEY="handler"
cluster=20
mem=1536

S3="infinicache"

echo "compiling lambda code..."
GOOS=linux go build $PWD/../lambda/$KEY.go
echo "compress file..."
zip $KEY $KEY
echo "updating lambda code.."

echo "putting code zip to s3"
aws s3api put-object --bucket ${S3} --key $KEY.zip --body $KEY.zip

go run $PWD/deploy_function.go -S3 ${S3} -create -config -prefix=$PREFIX -key=$KEY -to=$cluster -mem=$mem -timeout=$1
rm $KEY*
