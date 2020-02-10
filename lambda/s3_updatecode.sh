#!/bin/bash

PWD=`dirname $0`

PREFIX="Proxy1Node"
<<<<<<< HEAD:src/LambdaStore/s3_updatecode.sh
S3="ao.lambda.code"
=======
KEY="lambda"
>>>>>>> develop:lambda/s3_updatecode.sh
cluster=300
mem=1024

KEY="redeo_lambda"

echo "compiling lambda code..."
GOOS=linux go build $PWD/$KEY.go
echo "compress file..."
zip $KEY $KEY
echo "updating lambda code.."

echo "putting code zip to s3"
aws s3api put-object --bucket ${S3} --key $KEY.zip --body $KEY.zip

<<<<<<< HEAD:src/LambdaStore/s3_updatecode.sh
go run $PWD/../../sbin/deploy_function.go -S3 ${S3} -code=true -config=true -prefix=$PREFIX -vpc=true -key=$KEY -cluster=$cluster -mem=$mem -timeout=$1
=======
go run $PWD/../sbin/deploy_function.go -code=true -config=true -prefix=$PREFIX -vpc=true -key=$KEY -cluster=$cluster -mem=$mem -timeout=$1
>>>>>>> develop:lambda/s3_updatecode.sh
go clean
