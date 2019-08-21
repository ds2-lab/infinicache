package migrator

import (
	"errors"
	"fmt"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"net"
	"sync"
	"time"
)

const RemoteTotal int = 2

// ErrServerClosed is returned by the Server after a call to Shutdown or Close.
var ErrServerClosed = errors.New("proxy: Server closed")

// ErrServerListening is returned by the Server if server is listening.
var ErrServerListening = errors.New("proxy: Server is listening")

type Server struct {
	Addr    string // TCP address to listen on
	Verbose bool
	Debug   bool
	//ServingFeed channel.Out
	//ServedFeed  channel.Out
	Throttle chan bool

	mu        sync.RWMutex
	log       logger.ILogger
	remoteIds [RemoteTotal]int
	remotes   [RemoteTotal]*net.TCPAddr
	lPort     string
	rPort     string
	lListener *net.TCPListener
	rListener *net.TCPListener
	//lConn      net.Conn
	//rConn      net.Conn
	activeConn map[*forwardConnection]struct{}
	connId     uint64
	listening  chan struct{}
	done       chan struct{}
	//servingFeed     channel.Channel
	//servedFeed      channel.Channel
	remotePrimary   int
	remoteSecondary int

	started     time.Time // Time started listening
	requested   int32     // Number of incoming requests.
	served      int32     // Number of served requests.
	serving     int32     // Accurate serving requests.
	sumResponse int64

	// Accumulated response time.
	// usage          uint64       // Accumulated serve time in nanoseconds.
	// updated        uint64       // Last updated duration from started.
}

func (srv *Server) getLoggerLevel() int {
	logLevel := logger.LOG_LEVEL_INFO
	if srv.Debug {
		logLevel = logger.LOG_LEVEL_ALL
	}
	return logLevel
}

func NewServer(lport string, rport string, debug bool) *Server {
	srv := &Server{
		lPort:   lport,
		rPort:   rport,
		Addr:    fmt.Sprintf("lport:%d, rport:%d", lport, rport),
		Verbose: false,
		Debug:   debug,
	}
	if debug {
		srv.log = &logger.ColorLogger{
			Verbose: srv.Verbose,
			Level:   srv.getLoggerLevel(),
			Prefix:  "Proxy server ",
			Color:   true,
		}
	} else {
		srv.log = logger.NilLogger
	}
	srv.activeConn = make(map[*forwardConnection]struct{})
	srv.done = make(chan struct{})
	//srv.servingFeed = flash.NewChannel()
	//srv.ServingFeed = srv.servingFeed
	//srv.servedFeed = flash.NewChannel()
	//srv.ServedFeed = srv.servedFeed
	srv.Throttle = make(chan bool, 10)

	return srv
}

func (srv *Server) Forward() {
	laddr, _ := net.ResolveTCPAddr("tcp", srv.lPort)
	raddr, _ := net.ResolveTCPAddr("tcp", srv.rPort)
	srv.lListener, _ = net.ListenTCP("tcp", laddr)
	srv.rListener, _ = net.ListenTCP("tcp", raddr)
	srv.log.Debug("start listening on %v", laddr)
	srv.log.Debug("start listening on %v", raddr)
	defer srv.lListener.Close()
	defer srv.rListener.Close()

	for {
		lConn, _ := srv.lListener.AcceptTCP()
		rConn, _ := srv.rListener.AcceptTCP()
		srv.log.Debug("lConn is %v, rConn is %v", lConn.RemoteAddr(), rConn.RemoteAddr())

		forward := newForwardConnection(lConn, laddr, rConn, raddr)
		forward.Debug = srv.Debug
		//forward.Nagles = true
		forward.log = &logger.ColorLogger{
			Verbose: srv.Verbose,
			Level:   srv.getLoggerLevel(),
			//Prefix:      fmt.Sprintf("Connection #%03d ", connid),
			Color: true,
		}
		//forward.pipe(lConn, rConn)
		go forward.pipe(lConn, rConn)
		go forward.pipe(rConn, lConn)
		fmt.Println("pipe complete")

	}

}

func (srv *Server) trackConn(fconn *forwardConnection, add bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if add {
		srv.activeConn[fconn] = struct{}{}
	} else {
		delete(srv.activeConn, fconn)
	}
}
