#!/bin/bash

function update_lambda_timeout() {
    NAME=$1
    TIME=$2
    echo "updating lambda store timeout"
    for i in {0..13}
    do
#            aws lambda update-function-code --function-name $prefix$i --zip-file fileb://Lambda2SmallJPG.zip
#            aws lambda update-function-configuration --function-name $prefix$i --memory-size $mem
        aws lambda update-function-configuration --function-name $NAME$i --timeout $TIME
#            aws lambda update-function-configuration --function-name $prefix$name$i --handler redeo_lambda
#            aws lambda put-function-concurrency --function-name $name$i --reserved-concurrent-executions $concurrency
    done
}

function start_proxy() {
    EC=$1
    MEM=$2
    OBJSZ=$3
    NO=$4
    echo "running proxy server"
    echo "_"$NO"_"$EC"_"lambda$MEM"_"$OBJSZ
#    -prefix="_"$NO"_"$EC"_"lambda$MEM"_"$OBJSZ
    GOMAXPROCS=36 go run ~/project/lambdaStore/server.go -replica=false -isPrint=true
}

function bench() {
    N=$1
    C=$2
    KEYMIN=$3
    KEYMAX=$4
    SZ=$5
    D=$6
    P=$7
    OP=$8
    go run ~/project/src/github.com/tddg/redbench/bench.go -addrlist localhost:6378 -n $N -c $C -keymin $KEYMIN -keymax $KEYMAX -sz $SZ -d $D -p $P -op $OP -dec
}