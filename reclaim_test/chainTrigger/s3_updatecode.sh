#!/bin/bash

PWD=`dirname $0`
PREFIX="chain"
KEY="chain"
cluster=300
mem=3008

echo "compiling lambda code..."
GOOS=linux go build $KEY.go
echo "compress file..."
zip $KEY $KEY
echo "updating lambda code.."

echo "putting code zip to s3"
aws s3api put-object --bucket ao.lambda.code --key $KEY.zip --body $KEY.zip

go run $PWD/../../sbin/deploy_function.go -from 0 -to 300 -code=true -config=true -prefix=$PREFIX -vpc=true -key=$KEY -mem=$mem -timeout=$1
go clean
rm $KEY.zip