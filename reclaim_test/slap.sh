#!/bin/bash

go run expand.go -prefix 3008Mb_1 -name reclaim -count 1000 -m 1
echo 1min finish
go run expand.go -prefix 3008Mb_2 -name reclaim -count 1000 -m 2
echo 2min finish
go run expand.go -prefix 3008Mb_3 -name reclaim -count 1000 -m 3
echo 3min finish
go run expand.go -prefix 3008Mb_5 -name reclaim -count 1000 -m 5
echo 5min finish
go run expand.go -prefix 3008Mb_10 -name reclaim -count 1000 -m 10
echo 10min finish
go run expand.go -prefix 3008Mb_15 -name reclaim -count 1000 -m 15
echo 15min finish
