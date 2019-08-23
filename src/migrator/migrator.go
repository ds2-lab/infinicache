package migrator

import (
	"errors"
	"fmt"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"net"
)

// ErrServerClosed is returned by the Server after a call to Shutdown or Close.
var ErrServerClosed = errors.New("migrator: Server closed")

type Server struct {
	Addr    string // TCP address to listen on
	Verbose bool
	Debug   bool
	LastError error

	log       logger.ILogger
	port      int
	listener  *net.TCPListener
}

func New(port int, debug bool) *Server {
	srv := &Server{
		port:    port,
		Addr:    fmt.Sprintf(":%d", port),
		Verbose: false,
		Debug:   debug,
	}
	srv.log = &logger.ColorLogger{
		Verbose: srv.Verbose,
		Level:   srv.getLoggerLevel(),
		Prefix:  fmt.Sprintf("Migrator %s ", port),
		Color:   true,
	}

	return srv
}

func (srv *Server) getLoggerLevel() int {
	logLevel := logger.LOG_LEVEL_INFO
	if srv.Debug {
		logLevel = logger.LOG_LEVEL_ALL
	}
	return logLevel
}

func (srv *Server) Listen() (err error) {
	addr, _ := net.ResolveTCPAddr("tcp", srv.Addr)
	srv.listener, err = net.ListenTCP("tcp", addr)
	if err != nil {
		srv.log.Error("Failed to listen on %v", srv.Addr)
		srv.LastError = err
		return
	}

	srv.log.Info("Start listening on %v", srv.Addr)
	return
}

func (srv *Server) Serve() {
	defer srv.listener.Close()

	lConn, err := srv.listener.AcceptTCP()
	if err != nil {
		srv.log.Error("Error on accept 1st incoming connection: %v", err)
		srv.LastError = err
		return
	}
	srv.log.Debug("Source lambda connected: %v", lConn.RemoteAddr())

	rConn, err := srv.listener.AcceptTCP()
	if err != nil {
		srv.log.Error("Error on accept 2nd incoming connection: %v", err)
		srv.LastError = err
		return
	}
	srv.log.Debug("Destination lambda connected: %v", rConn.RemoteAddr())

	fConn := newForwardConnection(lConn, rConn)
	fConn.Debug = srv.Debug
	//fConn.Nagles = true
	fConn.log = srv.log

	fConn.forward()
}
