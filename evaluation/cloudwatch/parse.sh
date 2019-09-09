#!/bin/bash

for file in cost/*
do
    rm ${file}/aws-logs-write-test
    for zip in ${file}/*/*/*
    do
        gunzip ${zip}
        echo $zip
    done
done

for file in cost/*
do
	for zip in ${file}/*/*/*
    do
		cat ${zip} >> ${file}.dat
	done
done
