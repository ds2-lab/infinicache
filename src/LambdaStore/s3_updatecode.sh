#!/bin/bash

PREFIX="Proxy1Node"
if [ "$1" != "" ] ; then
  PREFIX="$1"
fi
# concurrency=30

echo "compiling lambda code..."
GOOS=linux go build redeo_lambda.go
echo "compress file..."
zip Lambda2SmallJPG redeo_lambda
echo "updating lambda code.."

echo "putting code zip to s3"
aws s3api put-object --bucket ao.lambda.code --key lambdastore.zip --body Lambda2SmallJPG.zip

for i in {0..63}
do
     aws lambda update-function-code --function-name $PREFIX$i --s3-bucket ao.lambda.code --s3-key lambdastore.zip
     # aws lambda update-function-configuration --function-name $PREFIX$i --memory-size $mem
     # aws lambda update-function-configuration --function-name $PREFIX$i --timeout $2
#    aws lambda update-function-configuration --function-name $PREFIX$i --handler redeo_lambda
#    aws lambda put-function-concurrency --function-name $PREFIX$i --reserved-concurrent-executions $concurrency
done

go clean
