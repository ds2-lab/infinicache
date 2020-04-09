#!/bin/bash

PREFIX=$1

for ((i = 0; i <= $2; i++)); do
  aws lambda delete-function --function-name $PREFIX$i
done



