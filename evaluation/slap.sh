#!/bin/bash

source util.sh

function perform(){
    for i in {0..4}
    do
        update_lambda_timeout Node $((i*100 + 200))
        wait
        start_proxy 4+2 128 1048576 1 &
    #    wait
        sleep 10s
    #    set
        bench 1 2 1 10 1048576 4 2 0
        sleep 10s
        bench 1 2 1 10 1048576 4 2 1
    #    kill server pid
        server_id=`ps -ef | grep server | grep -v "grep" | awk '{print $2}'`
        echo $server_id
        for id in $server_id
            do
            kill -9 $id
            echo "killed $id"
        done
    done
}

perform