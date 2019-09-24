#!/bin/bash

LOGSET=$1

# Latency
cat ${LOGSET}.log | grep "EcRedis" | grep -E "(Set|Got) v2" | awk '{print $1" "$2","$4","$6}' | awk -F \( '{print $1}' | sed 's/,[^G]*Got/,get/;s/,[^S]*Set/,set/;s/\([0-9]m*s\).*$/\1/' > ${LOGSET}_latency.csv
