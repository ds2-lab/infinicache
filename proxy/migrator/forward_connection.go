package migrator

import (
	"io"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"net"
	"sync"

)

// forwardConnection - Manages a forwardConnection connection, piping data between local and remote.
type forwardConnection struct {
	sentBytes     uint64
	receivedBytes uint64
	lconn         io.ReadWriteCloser
	rconn         io.ReadWriteCloser
	closed        chan bool
	traceFormat   string
	mu            sync.Mutex
	log           logger.ILogger

	// Settings
	Nagles bool
	Debug  bool
	Binary bool
}

// New - Create a new forwardConnection instance. Takes over local connection passed in,
// and closes it when finished.
func newForwardConnection(lconn *net.TCPConn, rconn *net.TCPConn) *forwardConnection {
	return &forwardConnection{
		lconn:  lconn,
		rconn:  rconn,
		closed: make(chan bool),
		log:    logger.NilLogger,
	}
}

type setNoDelayer interface {
	SetNoDelay(bool) error
}

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
func (fconn *forwardConnection) forward() {

	//Nagles?
	if fconn.Nagles {
		if conn, ok := fconn.lconn.(setNoDelayer); ok {
			conn.SetNoDelay(true)
		}
		if conn, ok := fconn.rconn.(setNoDelayer); ok {
			conn.SetNoDelay(true)
		}
	}

	// Reset format for trace
	if fconn.Binary {
		fconn.traceFormat = "%x"
	} else {
		fconn.traceFormat = "%s"
	}

	// Bidirectional copy
	go fconn.pipe(fconn.lconn, fconn.rconn)
	go fconn.pipe(fconn.rconn, fconn.lconn)

	// Wait for close...
	<-fconn.closed
	fconn.log.Info("Closed (%d bytes sent, %d bytes recieved)", fconn.sentBytes, fconn.receivedBytes)
}

func (fconn *forwardConnection) pipe(src io.Reader, dst io.Writer) {
	islocal := src == fconn.lconn

	// Directional copy (64k buffer)
	buff := make([]byte, 0xffff)
	for {
		n, readErr := src.Read(buff)
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

		//show output
		fconn.trace(islocal, b, n)

		//write out result
		n, writeErr := dst.Write(b)
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
		fconn.log.Trace(">>> %d bytes sent", len)
		fconn.log.Trace(fconn.traceFormat, bytes)
	} else {
		fconn.log.Trace("<<< %d bytes recieved", len)
		fconn.log.Trace(fconn.traceFormat, bytes)
	}
}
