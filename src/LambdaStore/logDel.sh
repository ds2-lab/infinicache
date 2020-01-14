# deleting log in cloudWatch with prefix
#!/bin/bash

prefix=$1

for ((i = 0; i <= $2; i++)); do
    aws logs delete-log-group --log-group-name /aws/lambda/$prefix$i
done
