#!/bin/sh
LAMBDA="/aws/lambda/reclaim"
FILE="aws/lambda/reclaim"

start='2019-09-06 00:00:00'
end='2019-09-09 00:00:00'

# Convert date into seconds (Format is %s)
startTime=$(date  -j -f "%Y-%m-%d %H:%M:%S" "$start" +%s)000
endTime=$(date  -j -f "%Y-%m-%d %H:%M:%S" "$end" +%s)000


for i in {0..5}
do
    aws logs create-export-task --log-group-name $LAMBDA$i --from ${startTime} --to ${endTime} --destination "ao.cost.log" --destination-prefix $FILE$i
    sleep 2s
done