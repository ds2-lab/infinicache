#!/bin/bash

PWD=`dirname $0`
LOGSET=cost
if [ "$1" != "" ] ; then
  LOGSET=$1
fi

for file in $PWD/$LOGSET/*
do
    rm ${file}/aws-logs-write-test
    for zip in ${file}/*/*/*
    do
        gunzip ${zip}
        echo $zip
    done
done

for file in $PWD/$LOGSET/*
do
	for zip in ${file}/*/*/*
    do
		cat ${zip} >> $LOGSET.dat
	done
done

echo "cat $PWD/$LOGSET.dat | grep -E 'START|New lambda invocation|REPORT' | sed '/START/{ s/^.*$/1 2 3 4 5 invocation:unknown/;h;d;};/invocation/{s/\(invocation:\) \(.*\)/\1\2/;h;d;};/REPORT/{H;g;s/\n/ /;}' | awk '{print $7","$6","$10","$12","$16}' > $PWD/${LOGSET}_bill.csv"
cat $PWD/$LOGSET.dat | grep -E 'START|New lambda invocation|REPORT' | sed '/START/{ s/^.*$/1 2 3 4 5 invocation:unknown/;h;d;};/invocation/{s/\(invocation:\) \(.*\)/\1\2/;h;d;};/REPORT/{H;g;s/\n/ /;}' | awk '{print $7","$6","$10","$12","$16}' > $PWD/${LOGSET}_bill.csv
