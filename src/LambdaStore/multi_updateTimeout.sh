#!/bin/bash

prefix=$1

for i in {0..13}
do
  aws lambda update-function-configuration --function-name $prefix$name$i --timeout $2
done


