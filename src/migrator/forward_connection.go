package migrator

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/wangaoone/LambdaObjectstore/lib/logger"
)

// forwardConnection - Manages a forwardConnection connection, piping data between local and remote.
type forwardConnection struct {
	sentBytes     uint64
	receivedBytes uint64
	laddr         *net.TCPAddr
	raddr         *net.TCPAddr
	lconn         io.ReadWriteCloser
	rconn         io.ReadWriteCloser
	closed        chan bool
	traceFormat   string
	mu            sync.Mutex
	log           logger.ILogger
	requested     time.Time

	Matcher  func(*forwardConnection, bool, []byte)
	Replacer func([]byte) []byte

	// Settings
	Nagles bool
	Debug  bool
	Binary bool
}

// New - Create a new forwardConnection instance. Takes over local connection passed in,
// and closes it when finished.
func newForwardConnection(lconn *net.TCPConn, laddr *net.TCPAddr, rconn *net.TCPConn, raddr *net.TCPAddr) *forwardConnection {
	return &forwardConnection{
		lconn:  lconn,
		laddr:  laddr,
		rconn:  rconn,
		raddr:  raddr,
		closed: make(chan bool),
		log:    logger.NilLogger,
	}
}

type setNoDelayer interface {
	SetNoDelay(bool) error
}

// Start - open connection to remote and start forwardConnectioning data.
//func (fconn *forwardConnection) forward(srv *Server) {
//	var err error
//	// Connect to remotes
//	//for i, raddr := range fconn.raddrs {
//	//	fconn.rconns[i], err = net.DialTCP("tcp", nil, raddr)
//	//	if err != nil {
//	//		fconn.log.Warn("Remote connection failed: %s", err)
//	//		return
//	//	}
//	//	defer fconn.rconns[i].Close()
//	//}
//
//	srv.trackConn(fconn, true)
//
//	// Nagles?
//	if fconn.Nagles {
//		if conn, ok := fconn.lconn.(setNoDelayer); ok {
//			conn.SetNoDelay(true)
//		}
//		for _, rconn := range fconn.rconns {
//			if conn, ok := rconn.(setNoDelayer); ok {
//				conn.SetNoDelay(true)
//			}
//		}
//	}
//
//	// Display both ends
//	for _, raddr := range fconn.raddrs {
//		fconn.log.Info("Opened %s >>> %s", fconn.laddr.String(), raddr.String())
//	}
//
//	// Reset format for trace
//	if fconn.Binary {
//		fconn.traceFormat = "%x"
//	} else {
//		fconn.traceFormat = "%s"
//	}
//
//	// Bidirectional copy
//	rwriter := fconn.rconns[0].(io.Writer);
//	rreader := fconn.rconns[0].(io.Reader);
//	if len(fconn.rconns) > 1 {
//		rwriter = MultiWriter(fconn.rconnWriters()...)
//		rreader = MultiReader(fconn.rconnReaders()...)
//	}
//	go fconn.pipe(fconn.lconn, rwriter)
//	go fconn.pipe(rreader, fconn.lconn)
//
//	// Wait for close...
//	<-fconn.closed
//	fconn.log.Info("Closed (%d bytes sent, %d bytes recieved)", fconn.sentBytes, fconn.receivedBytes)
//	srv.trackConn(fconn, false)
//}

func (fconn *forwardConnection) close() {
	fconn.mu.Lock()
	defer fconn.mu.Unlock()

	select {
	case <-fconn.closed:
		// Already closed. Don't close again.
	default:
		close(fconn.closed)
	}
}

func (fconn *forwardConnection) isClosed() <-chan bool {
	fconn.mu.Lock()
	defer fconn.mu.Unlock()
	return fconn.closed
}

func (fconn *forwardConnection) err(s string, err error) {
	if err != io.EOF {
		fconn.log.Warn(s, err)
	} else {
		fconn.log.Debug(s, err)
	}

	fconn.close()
}

// Start - open connection to remote and start forwardConnectioning data.
func (fconn *forwardConnection) forward(srv *Server) {

	//Nagles?
	if fconn.Nagles {
		if conn, ok := fconn.lconn.(setNoDelayer); ok {
			conn.SetNoDelay(true)
		}
		if conn, ok := fconn.rconn.(setNoDelayer); ok {
			conn.SetNoDelay(true)
		}
	}

	// Display both ends
	fconn.log.Info("Opened %s >>> %s", fconn.laddr.String(), fconn.raddr.String())

	// Reset format for trace
	if fconn.Binary {
		fconn.traceFormat = "%x"
	} else {
		fconn.traceFormat = "%s"
	}

	// Bidirectional copy
	rwriter := fconn.rconn.(io.Writer);
	rreader := fconn.rconn.(io.Reader);
	//if len(fconn.rconn) > 1 {
	//	rwriter = MultiWriter(fconn.rconnWriters()...)
	//	rreader = MultiReader(fconn.rconnReaders()...)
	//}
	go fconn.pipe(fconn.lconn, rwriter)
	go fconn.pipe(rreader, fconn.lconn)

	// Wait for close...
	<-fconn.closed
	fconn.log.Info("Closed (%d bytes sent, %d bytes recieved)", fconn.sentBytes, fconn.receivedBytes)
	srv.trackConn(fconn, false)
}

func (fconn *forwardConnection) pipe(src io.Reader, dst io.Writer) {
	islocal := src == fconn.lconn

	// Directional copy (64k buffer)
	// Using double caching to buy time for matcher
	buffs := [5][]byte{}
	len := len(buffs)
	pivot := 0
	ready := make(chan []byte, len)
	// Fill channel with buffers, and later filling depends on matcher.
	for ; pivot < len; pivot++ {
		buffs[pivot] = make([]byte, 0xffff)
		ready <- buffs[pivot]
	}

	var buff []byte
	for {
		select {
		case buff = <-ready:
		default:
			// Warn and wait
			fconn.log.Warn("It takes too long to call matcher")
			buff = <-ready
		}

		n, readErr := src.Read(buff)
		fmt.Println("n1 is", n)
		if readErr != nil {
			select {
			case <-fconn.isClosed():
				// Stop on closing
				return
			default:
			}
			if readErr == io.EOF && n > 0 {
				// Pass down to to transfer rest bytes
			} else if islocal {
				fconn.err("Inbound read failed \"%s\"", readErr)
				return
			} else {
				fconn.err("Outbound read failed \"%s\"", readErr)
				return
			}
		}
		b := buff[:n]

		//execute replace
		if fconn.Replacer != nil {
			b = fconn.Replacer(b)
		}

		//execute match
		if fconn.Matcher != nil {
			go func() {
				fconn.Matcher(fconn, islocal, b)
				pivot++
				ready <- buffs[pivot%len]
			}()
		} else {
			pivot++
			ready <- buffs[pivot%len]
		}

		fmt.Println(islocal, b)
		//show output
		fconn.trace(islocal, b, n)

		//write out result
		n, writeErr := dst.Write(b)
		fmt.Println("n2 is", n)

		if writeErr != nil {
			if islocal {
				fconn.err("Inbound write failed \"%s\"", writeErr)
			} else {
				fconn.err("Outbound write failed \"%s\"", writeErr)
			}
			return
		}
		if islocal {
			fconn.sentBytes += uint64(n)
		} else {
			fconn.receivedBytes += uint64(n)
		}

		if (readErr != nil) {
			fconn.close()
			return
		}
	}
}

func (fconn *forwardConnection) trace(islocal bool, bytes []byte, len int) {
	if !fconn.Debug {
		return
	}

	if islocal {
		fconn.log.Debug(">>> %d bytes sent", len)
		fconn.log.Trace(fconn.traceFormat, bytes)
	} else {
		fconn.log.Debug("<<< %d bytes recieved", len)
		fconn.log.Trace(fconn.traceFormat, bytes)
	}
}

func (fconn *forwardConnection) markRequest(id string) {
	fconn.requested = time.Now()
}

func (fconn *forwardConnection) markResponse(id string) float64 {
	return time.Since(fconn.requested).Seconds()
}
