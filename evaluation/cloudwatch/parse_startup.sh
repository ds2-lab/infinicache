#!/bin/bash

PWD=`dirname $0`
LOGSET=cost
if [ "$1" != "" ] ; then
  LOGSET=$1
fi

SKIPUNZIP=$2

if [ "$SKIPUNZIP" == "" ]; then
    for file in $LOGSET/*
    do
        rm ${file}/aws-logs-write-test
        for zip in ${file}/*/*/*
        do
            gunzip ${zip}
            echo $zip
        done
    done
fi

echo "exporting ${PWD}/${LOGSET}_startup.csv"
for file in $LOGSET/*
do
	for zip in ${file}/*/*/*
    do
		TS=`head -n 1 ${zip} | awk '{print $1}'`
        FUNC=`echo "$zip" | awk -F / '{print $4}'`
        echo "$FUNC,$TS" >> ${PWD}/${LOGSET}_startup.csv
	done
done
