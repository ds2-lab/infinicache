#!/bin/bash

if [ "$GOPATH" == "" ] ; then
  echo "No \$GOPATH defined. Install go and set \$GOPATH first."
fi

PWD=`dirname $0`
DATE=`date "+%Y%m%d%H%M"`
ENTRY="/data/$DATE"
NODE_PREFIX="Store1VPCNode"

source $PWD/util.sh

function perform(){
	FILE=$1
	CLUSTER=$2
	SCALE=$3
	COMPACT=$4

	PREPROXY=$PWD/$ENTRY/simulate-$CLUSTER$COMPACT

	start_proxy $PREPROXY &
  # Wait for proxy is ready
	while [ ! -f /tmp/lambdaproxy.pid ]
	do
		sleep 1s
	done
	cat /tmp/lambdaproxy.pid
	#        set
	sleep 1s
	playback 10 2 $SCALE $CLUSTER $FILE $COMPACT
	kill -2 `cat /tmp/lambdaproxy.pid`
  # Wait for proxy cleaned up
  while [ -f /tmp/lambdaproxy.pid ]
	do
		sleep 1s
	done
}

mkdir -p $PWD/$ENTRY

START=`date +"%Y-%m-%d %H:%M:%S"`
perform $1 $2 $3 $4
mv $PWD/log $PWD/$ENTRY.log
END=`date +"%Y-%m-%d %H:%M:%S"`

echo "Transfering logs from CloudWatch to S3: $START - $END ..."
cloudwatch/export_ubuntu.sh $DATE/ "$START" "$END"
