#!/bin/bash

name="Lambda2SmallJPG"
mem=3008
concurrency=13

echo "compiling lambda code..."
GOOS=linux go build redeo_lambda.go
echo "compress file..."
zip Lambda2SmallJPG redeo_lambda
echo "updating lambda code.."
# aws lambda update-function-code --function-name dataNode0 --zip-file fileb://Lambda2SmallJPG.zip
# aws lambda update-function-code --function-name dataNode1 --zip-file fileb://Lambda2SmallJPG.zip
aws lambda update-function-code --function-name $name --zip-file fileb://Lambda2SmallJPG.zip
aws lambda update-function-configuration --function-name $name --memory-size $mem
aws lambda put-function-concurrency --function-name $name --reserved-concurrent-executions $concurrency

go clean