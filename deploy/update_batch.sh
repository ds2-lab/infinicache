#!/bin/bash

BASE=`pwd`/`dirname $0`
PREFIX="ElasticMB"
KEY="lambda"
group=9
# try -code

S3="mason-leap-lab.infinicache"
EMPH="\033[1;33m"
RESET="\033[0m"

if [ "$2" == "" ] ; then
    CODE=""
elif [ "$2" == "-code" ] ; then
    CODE="$2"
else
    CODE="$3"
    PREFIX="$2"
fi

if [ "$CODE" == "-code" ] ; then
    echo -e "Updating "$EMPH"code and configuration"$RESET" of Lambda deployments ${PREFIX}0- to ${PREFIX}$group- to $1s timeout..."
    read -p "Press any key to confirm, or ctrl-C to stop."

    cd $BASE/../lambda
    echo "Compiling lambda code..."
    GOOS=linux go build
    echo "Compressing file..."
    zip $KEY $KEY
    echo "Putting code zip to s3"
    aws s3api put-object --bucket ${S3} --key $KEY.zip --body $KEY.zip
else 
    echo -e "Updating "$EMPH"configuration"$RESET" of Lambda deployments ${PREFIX}0- to ${PREFIX}$group- to $1s timeout..."
    read -p "Press any key to confirm, or ctrl-C to stop."
fi

echo "Updating Lambda deployments..."
for i in $(seq 0 1 $group)
do
  echo "update_function.sh $1 $PREFIX$i- $CODE"
  ./update_function.sh $1 $PREFIX$i- $CODE
done

if [ "$CODE" == "-code" ] ; then
  rm $KEY*
fi

