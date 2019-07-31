#!/bin/bash

source util.sh

OPTIND=1         # Reset in case getopts has been used previously in the shell.
MEM=128
NUMBER=1
CON=1
KEYMIN=1
KEYMAX=2
SZ=128
DATA=4
PARITY=2
TIME=$1
while getopts ":hm:n:c:a:b:s:d:p:t:" opt; do
    case ${opt} in
    h)
#        echo "grade [options] tarfile"
#        echo "options:"
#        echo "  -m move file to the sub directory, which is named by 'date', of 'grade' program"
        exit 0
        ;;
    m)  MEM=$OPTARG
        ;;
    n)  NUMBER=$OPTARG
        ;;
    c)  CON=$OPTARG
        ;;
    a)  KEYMIN=$OPTARG
        ;;
    b)  KEYMAX=$OPTARG
        ;;
    s)  SZ=$OPTARG
        ;;
    d)  DATA=$OPTARG
        ;;
    p)  PARITY=$OPTARG
        ;;
    t)  TIME=$OPTARG
        ;;
    \?)
        ;;
    esac
done
shift $((OPTIND-1))

function perform(){
    MEM=$1
    NUMBER=$2
    CON=$3
    KEYMIN=$4
    KEYMAX=$5
    SZ=$6
    DATA=$7
    PARITY=$8
    TIME=$9
#    echo $i"_"DATA$P"_"lambda$MEM"_"$SZ

    for i in {1..2}
    do
        PREPROXY=No.$i"_"$DATA"_"$PARITY"_"lambda$MEM"_"$SZ
        PRESET=No.$i"_"$DATA"_"$PARITY"_"lambda$MEM"_"$SZ"_SET"
        PREGET=No.$i"_"$DATA"_"$PARITY"_"lambda$MEM"_"$SZ"_GET"

        update_lambda_timeout Node $((TIME+i*10))
        wait
        start_proxy $PREPROXY &
        while [ ! -f /tmp/pidLog.txt ]
        do
            sleep 1s
        done
        cat /tmp/pidLog.txt
#        set
        sleep 5s
        bench $NUMBER $CON $KEYMIN $KEYMAX $SZ $DATA $PARITY 0 $PRESET
#        while [ ! -f /var/run/pidLog.txt ]
#        do
#            sleep 1s
#        done
        sleep 1s
#        get
        bench $NUMBER $CON $KEYMIN $KEYMAX $SZ $DATA $PARITY 1 $PREGET
        kill -9 `cat /tmp/pidLog.txt`
    done
}

#perform $*
#perform

DS=(4 5 10 10 10)
PS=(2 1 1 2 4)
C=(1)
N=(3)

for mem in 128 256 512 1024 1536 2048 3008
do
    update_lambda_mem Node $mem
    for sz in 10485760 20971520 # 41943040 62914020 83886080 104857600
    do
        for k in {0..4}
        do
            for l in 0
            do
                perform $mem ${N[l]} ${C[l]} 1 2 $sz ${DS[k]} ${PS[k]} 150
#                perform $mem 1 3 1 2 $sz ${DS[k]} ${PS[k]} 200
            done
        done
    done
done
