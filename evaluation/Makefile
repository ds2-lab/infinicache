deploy:
	aws configure
	aws s3api put-bucket-policy --bucket tianium.default --policy file:///home/ubuntu/go/src/github.com/wangaoone/LambdaObjectstore/evaluation/cloudwatch/policy.json

microbench:
	./slap.sh 1>./log 2>&1 &

simulate:
	./playback.sh /trace/docker_traces/data_centers/csv/lon02_10mb_1d.csv 400 0.1 -compact 1>./log 2>&1 &

playback:
	./playback.sh /trace/docker_traces/data_centers/csv/lon02_10mb_1d.csv 400 0.1 1>./log 2>&1 &

start-server:
	./server.sh 1>./log 2>&1 &

stop-server:
	kill -2 $(shell cat /tmp/lambdaproxy.pid)
