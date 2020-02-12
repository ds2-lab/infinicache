deploy:
	aws configure
	
build:
	go build -o proxy/proxy github.com/mason-leap-lab/infinicache/proxy

start: build
	proxy/proxy 1>./log 2>&1 &

stop:
	kill -2 $(shell cat /tmp/infinicache.pid)
