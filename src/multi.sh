#!/bin/bash

name="Node"
mem=3008
# concurrency=30

echo "compiling lambda code..."
GOOS=linux go build redeo_lambda.go
echo "compress file..."
zip Lambda2SmallJPG redeo_lambda
echo "updating lambda code.."

for i in {0..13}
do
	aws lambda update-function-code --function-name $name$i --zip-file fileb://Lambda2SmallJPG.zip
	# aws lambda update-function-configuration --function-name $name$i --memory-size $mem
	# aws lambda update-function-configuration --function-name $name$i --timeout 200
	# aws lambda update-function-configuration --function-name $name$i--handler "redeo_lambda"
	# aws lambda put-function-concurrency --function-name $name$i --reserved-concurrent-executions $concurrency
done

go clean