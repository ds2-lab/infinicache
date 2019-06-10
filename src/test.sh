#!/bin/bash

echo "updating lambda timeout config..."
aws lambda update-function-configuration --function-name Lambda2SmallJPG --timeout $1
echo "running server..."
go run server.go