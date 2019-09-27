#!/bin/bash

LOGSET=$1

# Latency
cat ${LOGSET}.log | grep "EcRedis" | grep -E "(Set|Got) v2" | awk '{print $1" "$2","$4","$6}' | awk -F \( '{print $1}' | sed 's/,[^G]*Got/,get/;s/,[^S]*Set/,set/;s/\([0-9]m*s\).*$/\1/' > ${LOGSET}_latency.csvsed -i bak '/[0-9]ms$/s/\([0-9]\)ms$/\1,1/;/[0-9]s$/s/\([0-9]\)s$/\1,1000/' 201909110650_latency.csv
sed -i .bak '/[0-9]ms$/s/\([0-9]\)ms$/\1,1/;/[0-9]s$/s/\([0-9]\)s$/\1,1000/' ${LOGSET}_latency.csv
