#!/bin/bash

prefix="reclaim"
# name="Node"

GOOS=linux go build lambda.go
zip Lambda2SmallJPG lambda

echo "Creating lambda functions..."

for i in {0..999}
do
	aws lambda create-function \
	--function-name $prefix$i \
	--runtime go1.x \
	--role arn:aws:iam::037862857942:role/Proxy1 \
	--handler lambda \
	--zip-file fileb://Lambda2SmallJPG.zip

done
go clean
