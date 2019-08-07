#!/bin/bash

prefix=$1

subnetIds=$2        #use comma to separate
securityGroupIds=$3 #use comma to separate


echo "updating lambda code.."

for i in {0..13}
do
	aws lambda update-function-configuration --function-name $prefix$i --vpc-config SubnetIds=$subnetIds,SecurityGroupIds=$securityGroupIds
done
