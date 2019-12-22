#!/bin/bash

PREFIX=$1
PWD=`dirname $0`

BASE=$PWD/data/logs/$PREFIX
mkdir -p BASE

aws s3 cp s3://tianium.default/log/$1 $BASE --recursive
