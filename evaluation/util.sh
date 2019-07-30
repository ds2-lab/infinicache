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
    echo "running proxy server"
    PREFIX=$1
    GOMAXPROCS=36 go run ~/project/lambdaStore/server.go -replica=false -isPrint=true -prefix=$PREFIX
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
    FILE=$9
    go run ~/project/src/github.com/tddg/redbench/bench.go -addrlist localhost:6378 -n $N -c $C -keymin $KEYMIN -keymax $KEYMAX -sz $SZ -d $D -p $P -op $OP -file $FILE -dec
}