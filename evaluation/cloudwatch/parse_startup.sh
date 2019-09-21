#!/bin/bash

LOGSET=cost
if [ "$1" != "" ] ; then
  LOGSET=$1
fi

for file in $LOGSET/*
do
    rm ${file}/aws-logs-write-test
    for zip in ${file}/*/*/*
    do
        gunzip ${zip}
        echo $zip
    done
done

for file in $LOGSET/*
do
	for zip in ${file}/*/*/*
    do
		TS=`head -n 1 ${zip} | awk '{print $1}'`
    FUNC=`echo "$zip" | awk -F / '{print $3}'`
    echo "$FUNC,$TS" >> ${LOGSET}_startup.csv
	done
done
