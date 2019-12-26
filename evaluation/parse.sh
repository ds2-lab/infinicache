#!/bin/bash

LOGSET=$1
COMMAND=$2

# Latency
if [ "$COMMAND" == "" -o "$COMMAND" == "latency" ]; then
	cat -v ${LOGSET}.log | grep "EcRedis" | grep -E "(Set|Got) v2" | awk '{print $1" "$2","$4","$5","$6}' | awk -F \( '{print $1}' | sed 's/,[^G]*Got/,get/;s/,[^S]*Set/,set/;s/\^\[\[0m$//' > ${LOGSET}_latency.csv
	# sed -i .bak 's/\([0-9]m*s\).*$/\1/;/[0-9]ms$/s/\([0-9]\)ms$/\1,1/;/[0-9]s$/s/\([0-9]\)s$/\1,1000/' ${LOGSET}_latency.csv
fi

# Reset
if [ "$COMMAND" == "" -o "$COMMAND" == "reset" ]; then
	cat -v ${LOGSET}.log | grep -E "Reset v2" | awk '{print $1" "$2","$3","$4}' | sed 's/,[^G]*Reset/,reset/;s/\.\^\[\[0m$//' > ${LOGSET}_reset.csv
fi

# Recovery
if [ "$COMMAND" == "" -o "$COMMAND" == "recover" ]; then
	cat -v ${LOGSET}.log | grep "EcRedis" | grep -E "recover" | awk '{print $1" "$2","$6","substr($0, index($0,$9))}' | sed 's/: \[\([ 0-9]*\)\]^\[\[0m$/,\1/' > ${LOGSET}_recover.csv
fi
