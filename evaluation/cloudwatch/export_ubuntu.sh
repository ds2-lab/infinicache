#!/bin/bash
LAMBDA="/aws/lambda/"
FILE="log/"
LOG_PREFIX="Store1VPCNode"

PREFIX=$1
start=$2
end=$3

# Convert date into seconds (Format is %s)
startTime=$(date -d "$start" +%s)000
endTime=$(date -d "$end" +%s)000


for i in {0..399}
do
    aws logs create-export-task --log-group-name $LAMBDA$LOG_PREFIX$i --from ${startTime} --to ${endTime} --destination "tianium.default" --destination-prefix $FILE$PREFIX$LOG_PREFIX$i
    sleep 2s
done
