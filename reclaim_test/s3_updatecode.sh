#!/usr/bin/env bash

# if [ "$1" != "" ] ; then
#  PREFIX="$1"
# fi
# concurrency=30

echo "compiling lambda code..."
GOOS=linux go build reclaim.go
echo "compress file..."
zip reclaim reclaim
echo "updating lambda code.."

echo "putting code zip to s3"
aws s3api put-object --bucket ao.lambda.code --key reclaim.zip --body reclaim.zip

# for i in {0..299}
# do
#     aws lambda update-function-code --function-name $PREFIX$i --s3-bucket ao.lambda.code --s3-key reclaim.zip
#     # aws lambda update-function-configuration --function-name $PREFIX$i --handler reclaim
#     # aws lambda update-function-configuration --function-name $PREFIX$i --timeout $2
# #    aws lambda update-function-configuration --function-name $PREFIX$i --handler redeo_lambda
# #    aws lambda put-function-concurrency --function-name $PREFIX$i --reserved-concurrent-executions $concurrency
# done

go clean
