#!/usr/bin/env bash

PREFIX=$1
DATA=$1"_"dat
PWD=`dirname $0`

if [ "$1" == "" ] ; then
	echo "Please specify the data directory, in the form of YYYYMMDDHHmm"
	exit 1
fi

mkdir $DATA/
mkdir $DATA/$PREFIX"_"csv/
mkdir $DATA/$PREFIX"_"summary/
tar -zxvf $PWD/downloaded/$PREFIX.tar.gz -C $DATA/

for dat in $PWD/$DATA/$PREFIX/*.clog
do
    NAME=$(basename $dat .clog)
    inflate -f $PWD/$DATA/$PREFIX/$NAME.clog > $PWD/$DATA/$PREFIX"_"csv/${NAME}.csv
done

for dat in $PWD/$DATA/$PREFIX/*.txt
do
    NAME=$(basename $dat .txt)
    cp $PWD/$DATA/$PREFIX/$NAME.txt $PWD/$DATA/$PREFIX"_"summary/
done

mv $DATA $PWD/downloaded/