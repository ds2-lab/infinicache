# deleting log in cloudWatch with prefix
#!/bin/bash

prefix=$1

for i in {0..35}
do
    aws logs delete-log-group --log-group-name /aws/lambda/$prefix$i
done
