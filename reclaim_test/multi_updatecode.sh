#!/bin/bash

PREFIX="Proxy1Node"
if [ "$1" != "" ] ; then
  PREFIX="$1"
fi
mem=3008
# concurrency=30

echo "compiling lambda code..."
GOOS=linux go build redeo_lambda.go
echo "compress file..."
zip Lambda2SmallJPG redeo_lambda
echo "updating lambda code.."

for i in {0..13}
do
     aws lambda update-function-code --function-name $PREFIX$i --zip-file fileb://Lambda2SmallJPG.zip
     # aws lambda update-function-configuration --function-name $PREFIX$i --memory-size $mem
     # aws lambda update-function-configuration --function-name $PREFIX$i --timeout $2
#    aws lambda update-function-configuration --function-name $PREFIX$i --handler redeo_lambda
#    aws lambda put-function-concurrency --function-name $PREFIX$i --reserved-concurrent-executions $concurrency
done

go clean
