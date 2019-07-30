#!/bin/bash
#function aa(){
#    for i in {0..4}
#    do
#        echo $((i*100))
#    done
#    echo $1
#}
#
#function bb(){
#    aa $*
#
#}
num=12
echo "the number is: " $num
if [ `cat pid.txt` ];then
    echo "the unmber is larger than 10"
#elif [ $num -eq 10 ];then
#    echo "the number is equal with 10"
#else
#    echo "the number is smaller than 10"
fi