#!/usr/bin/env bash

aws iam create-user --user-name Collector

export S3POLICYARN=$(aws iam list-policies --query 'Policies[?PolicyName==`AmazonS3FullAccess`].{ARN:Arn}' --output text)
export CWLPOLICYARN=$( aws iam list-policies --query 'Policies[?PolicyName==`CloudWatchLogsFullAccess`].{ARN:Arn}' --output text)
aws iam attach-user-policy --user-name Collector --policy-arn $S3POLICYARN
aws iam attach-user-policy --user-name Collector --policy-arn $CWLPOLICYARN

aws iam list-attached-user-policies --user-name Collector