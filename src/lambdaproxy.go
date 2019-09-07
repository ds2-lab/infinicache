package main

import (
	"flag"
	"fmt"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/wangaoone/LambdaObjectstore/src/proxy"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/collector"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
)

var (
	replica       = flag.Bool("replica", false, "Enable lambda replica deployment")
	isPrint       = flag.Bool("isPrint", false, "Enable log printing")
	prefix        = flag.String("prefix", "log", "log file prefix")
	log           = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	lambdaLis net.Listener
	filePath  = "/tmp/lambdaproxy.pid"
)

func init() {
	global.Log = log
}

func main() {
	done := make(chan struct{}, 1)
	flag.Parse()

	// Register signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGABRT)

	// CPU profiling by default
	//defer profile.Start().Stop()

	global.Prefix = *prefix

	// Initialize collector
	collector.Create(global.Prefix)

	// Initialize log
	if *isPrint {
		log.Level = logger.LOG_LEVEL_ALL
	}

	log.Info("======================================")
	log.Info("replica: %v || isPrint: %v", *replica, *isPrint)
	log.Info("======================================")
	clientLis, err := net.Listen("tcp", fmt.Sprintf(":%d", global.BasePort))
	if err != nil {
		log.Error("Failed to listen clients: %v", err)
		os.Exit(1)
		return
	}
	lambdaLis, err = net.Listen("tcp", fmt.Sprintf(":%d", global.BasePort+1))
	if err != nil {
		log.Error("Failed to listen lambdas: %v", err)
		os.Exit(1)
		return
	}
	log.Info("Start listening to clients(port 6378) and lambdas(port 6379)")
	// initial proxy server
	srv := redeo.NewServer(nil)
	prxy := proxy.New(*replica)

	// config server
	srv.HandleStreamFunc("set", prxy.HandleSet)
	srv.HandleFunc("get", prxy.HandleGet)
	srv.HandleCallbackFunc(prxy.HandleCallback)

	// initiate lambda store proxy
	go prxy.Serve(lambdaLis)
	<-prxy.Ready()

	err = ioutil.WriteFile(filePath, []byte(fmt.Sprintf("%d", os.Getpid())), 0660)
	if err != nil {
		log.Warn("Failed to write PID: %v", err)
	}

	// Log goroutine
	//defer t.Stop()
	go func() {
		<-sig
		log.Info("Receive signal, killing server...")
		// done <- struct{}{}
		close(sig)

		collector.Stop()

		// Close server
		log.Info("Closing server...")
		srv.Close(clientLis)

		// Collect data
		log.Info("Collecting data...")
		prxy.CollectData()

		prxy.Close(lambdaLis)
		close(done)
	}()

	// Start serving (blocking)
	err = srv.ServeAsync(clientLis)
	if err != nil {
		select {
		case <-sig:
			// Normal close
		default:
			log.Error("Error on serve clients: %v", err)
		}
		srv.Release()
	}
	log.Info("Server closed.")

	// Wait for data collection
	<-done
	prxy.Release()
	proxy.CleanUpScheduler()

	err = os.Remove(filePath)
	if err != nil {
		log.Error("Failed to remove PID: %v", err)
	}
	os.Exit(0)
}
