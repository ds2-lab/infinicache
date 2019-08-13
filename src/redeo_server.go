package main

import (
	"flag"
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
)

var (
	replica       = flag.Bool("replica", true, "Enable lambda replica deployment")
	isPrint       = flag.Bool("isPrint", false, "Enable log printing")
	prefix        = flag.String("prefix", "log", "log file prefix")
	log           = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	lambdaLis    net.Listener
	filePath     = "/tmp/pidLog.txt"
)

func init() {
	global.Log = log
}

func logCreate() {
	// get local time
	//location, _ := time.LoadLocation("EST")
	// Set up nanoLog writer
	//nanoLogout, err := os.Create("/tmp/proxy/" + *prefix + "_proxy.clog")
	nanoLogout, err := os.Create(*prefix + "_proxy.clog")
	if err != nil {
		panic(err)
	}
	err = nanolog.SetWriter(nanoLogout)
	if err != nil {
		panic(err)
	}
}

func main() {
	done := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGABRT)
	//signal.Notify(sig, syscall.SIGKILL)
	//signal.Notify(sig, syscall.SIGINT)
	flag.Parse()
	// CPU profiling by default
	//defer profile.Start().Stop()
	// init log
	logCreate()
	if *isPrint {
		log.Level = logger.LOG_LEVEL_ALL
	}

	log.Info("======================================")
	log.Info("replica: %v || isPrint: %v", *replica, *isPrint)
	log.Info("======================================")
	clientLis, err := net.Listen("tcp", ":6378")
	if err != nil {
		log.Error("Failed to listen clients: %v", err)
		os.Exit(1)
		return
	}
	lambdaLis, err = net.Listen("tcp", ":6379")
	if err != nil {
		log.Error("Failed to listen lambdas: %v", err)
		os.Exit(1)
		return
	}
	log.Info("Start listening to clients(port 6378) and lambdas(port 6379)")
	// initial proxy and lambda server
	srv := redeo.NewServer(nil)

	// initial lambda store proxy
	prx := proxy.New(*replica)
	go prx.Serve(lambdaLis)

	err = ioutil.WriteFile(filePath, []byte(fmt.Sprintf("%d", os.Getpid())), 0660)
	if err != nil {
		log.Warn("Failed to write PID: %v", err)
	}
	log.Info("Proxy for lambda store is ready!")

	// Log goroutine
	//defer t.Stop()
	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-sig:
				log.Info("Receive signal, killing server...")
				close(sig)
				t.Stop()
				if err := nanolog.Flush(); err != nil {
					log.Error("Failed to save data: %v", err)
				}

				// Close server
				log.Info("Closing server...")
				srv.Close(clientLis)

				// Collect data
				log.Info("Collecting data...")
				for _, node := range global.Stores.All {
					global.DataCollected.Add(1)
					// send data command
					node.C() <- &redeo.ServerReq{ Cmd: "data" }
				}
				log.Info("Waiting data from Lambda")
				global.DataCollected.Wait()
				if err := nanolog.Flush(); err != nil {
					log.Error("Failed to save data from lambdas: %v", err)
				} else {
					log.Info("Data collected.")
				}

				prx.Close(lambdaLis)
				prx.Release()
				close(done)

				return
			case <-t.C:
				if time.Since(global.LastActivity) >= 10*time.Second {
					if err := nanolog.Flush(); err != nil {
						log.Warn("Failed to save data: %v", err)
					}
				}
			}
		}
	}()

	// Start serving (blocking)
	err = srv.MyServe(clientLis, global.Clients, global.Stores, global.NanoLog)
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
	err = os.Remove(filePath)
	if err != nil {
		log.Error("Failed to remove PID: %v", err)
	}
	os.Exit(0)
}
