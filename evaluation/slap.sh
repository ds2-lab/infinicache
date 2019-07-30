#!/bin/bash

source util.sh

function perform(){
    MEM=$0
    N=$1
    C=$2
    KEYMIN=$3
    KEYMAX=$4
    SZ=$5
    D=$6
    P=$7
    OP=$8
    FILE=$9

    for i in {0..4}
    do
        update_lambda_timeout Node $((i*100 + 200))
        wait
        start_proxy $i"_"$D$P"_"lambda$MEM"_"$SZ &
        while [ ! -f /var/run/pidLog.txt ]
        do
            sleep 1s
        done
        cat /var/run/pidLog.txt
#        set
        bench $N $C $KEYMIN $KEYMAX $SZ $D $P 0 $i"_"$D$P"_"lambda$MEM"_"$SZ"_SET.txt"
#        while [ ! -f /var/run/pidLog.txt ]
#        do
#            sleep 1s
#        done
        sleep 1s
#        get
        bench $N $C $KEYMIN $KEYMAX $SZ $D $P 1 $i"_"$D$P"_"lambda$MEM"_"$SZ"_GET.txt"
        kill -9 `cat /var/run/pidLog.txt`
    done
}

perform $*