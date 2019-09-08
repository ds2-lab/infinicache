deploy:
	aws configure

run:
	./slap.sh 1>./log 2>&1 &

start-server:
	./server.sh 1>./log 2>&1 &

stop-server:
	kill -2 $(shell cat /tmp/lambdaproxy.pid)
