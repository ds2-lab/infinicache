deploy:
	aws configure
	
build:
	go build -o proxy/proxy github.com/neboduus/infinicache/proxy

start: build
	proxy/proxy 1>./log 2>&1 &

stop:
	kill -2 $(shell cat /project/src/infinicache.pid)
