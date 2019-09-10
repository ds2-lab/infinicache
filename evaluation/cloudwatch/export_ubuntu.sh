#!/bin/sh
LAMBDA="/aws/lambda/Store1VPCNode"
FILE="/lambda/"

PREFIX=$1
start=$2
end=$3

# Convert date into seconds (Format is %s)
startTime=$(date -d "$start" +%s)000
endTime=$(date -d "$end" +%s)000


for i in {0..399}
do
    aws logs create-export-task --log-group-name $LAMBDA$i --from ${startTime} --to ${endTime} --destination "tianium.default" --destination-prefix $FILE$PREFIX$i
    sleep 1s
done
