#!/bin/bash

NATID=`aws ec2 describe-nat-gateways --filter 'Name=tag:Name,Values=nat-lambda' --filter 'Name=state,Values=available' | grep NatGatewayId | awk -F \" '{ print $4 }'`

if [ "$NATID" == "" ]; then
  echo "No qualified nat gateway found."
  exit
fi

echo "Deleting $NATID"
aws ec2 delete-nat-gateway --nat-gateway-id $NATID
