#!/bin/bash

echo "compiling lambda code..."
GOOS=linux go build redeo_lambda.go
echo "compress file..."
zip Lambda2SmallJPG redeo_lambda
echo "updating lambda code.."
aws lambda update-function-code --function-name Lambda2SmallJPG --zip-file fileb://Lambda2SmallJPG.zip
