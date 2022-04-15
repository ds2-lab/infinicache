#!/bin/bash

NATID=`aws ec2 describe-nat-gateways --filter 'Name=tag:Name,Values=nat-lambda' --filter 'Name=state,Values=available' | grep NatGatewayId | awk -F \" '{ print $4 }'`

if [ "$NATID" == "" ]; then

  aws ec2 create-nat-gateway --subnet-id subnet-77e0622b --allocation-id eipalloc-0fef6282d27e74209 --tag-specifications 'ResourceType=natgateway,Tags=[{Key=Name,Value=nat-lambda}]'

  for j in {0..2}
  do
    NATID=`aws ec2 describe-nat-gateways --filter 'Name=tag:Name,Values=nat-lambda' --filter 'Name=state,Values=available' | grep NatGatewayId | awk -F \" '{ print $4 }'`
    if [ "$NATID" == "" ]; then
      sleep 2s
    else
      break
    fi
  done

  # Abandon
  if [ "$NATID" == "" ]; then
    echo "Wait for nat gateway timeout. Failed to create the nat gateway"
    exit
  fi
fi

aws ec2 create-route --route-table-id rtb-04e86a30459311471 --destination-cidr-block 0.0.0.0/0 --nat-gateway-id $NATID
