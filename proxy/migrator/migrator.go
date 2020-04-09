package migrator

import (
	"errors"
	"fmt"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"github.com/cornelk/hashmap"
	"net"
	"time"
)

// ErrServerClosed is returned by the Server after a call to Shutdown or Close.
var ErrServerClosed = errors.New("migrator: Server closed")
var ListenTimeout = 30 * time.Second
var all = &hashmap.HashMap{}

type Server struct {
	Addr    string // TCP address to listen on
	Verbose bool
	Debug   bool
	LastError error

	log       logger.ILogger
	port      int
	listener  *net.TCPListener
	fconn     *forwardConnection
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
		Prefix:  fmt.Sprintf("Migrator %d ", port),
		Color:   true,
	}

	all.Set(port, srv)
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

	// Set timeout
	srv.listener.SetDeadline(time.Now().Add(ListenTimeout))

	srv.log.Info("Start listening on %v", srv.Addr)
	return
}

func (srv *Server) Serve() {
	defer srv.Close()

	lConn, err := srv.listener.AcceptTCP()
	if err != nil {
		srv.log.Error("Error on accept 1st incoming connection: %v", err)
		srv.LastError = err
		return
	}
	defer lConn.Close()
	srv.log.Debug("Source lambda connected: %v", lConn.RemoteAddr())

	rConn, err := srv.listener.AcceptTCP()
	if err != nil {
		srv.log.Error("Error on accept 2nd incoming connection: %v", err)
		srv.LastError = err
		return
	}
	defer rConn.Close()
	srv.log.Debug("Destination lambda connected: %v", rConn.RemoteAddr())

	srv.listener.Close()
	srv.listener = nil

	srv.fconn = newForwardConnection(lConn, rConn)
	srv.fconn.Debug = srv.Debug
	//fConn.Nagles = true
	srv.fconn.log = srv.log

	srv.fconn.forward()
	srv.fconn = nil

	all.Del(srv.port)
}

func (srv *Server) Close() {
	if srv.listener != nil {
		srv.listener.Close()
		srv.listener = nil
	}
	if srv.fconn != nil {
		srv.fconn.close()
		srv.fconn = nil
	}
}

func CleanUp() {
	for keyValue := range all.Iter() {
		keyValue.Value.(*Server).Close()
	}
}
