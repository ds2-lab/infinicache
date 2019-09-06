package lifetime

import (
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"math"
	"sync/atomic"
	"time"
)

const TICK = int64(100 * time.Millisecond)
// For Lambdas below 0.5vCPU(896M).
const TICK_1_ERROR_EXTEND = int64(10000 * time.Millisecond)
const TICK_1_ERROR = int64(10 * time.Millisecond)
// For Lambdas with 0.5vCPU(896M) and above.
const TICK_5_ERROR_EXTEND = int64(1000 * time.Millisecond)
const TICK_5_ERROR = int64(10 * time.Millisecond)
// For Lambdas with 1vCPU(1792M) and above.
const TICK_10_ERROR_EXTEND = int64(1000 * time.Millisecond)
const TICK_10_ERROR = int64(2 * time.Millisecond)

var (
	TICK_ERROR_EXTEND = TICK_10_ERROR_EXTEND
	TICK_ERROR = TICK_10_ERROR
)

func init() {
	// adapt
	if lambdacontext.MemoryLimitInMB < 896 {
		TICK_ERROR_EXTEND = TICK_1_ERROR_EXTEND
		TICK_ERROR = TICK_1_ERROR
	} else if lambdacontext.MemoryLimitInMB < 1792 {
		TICK_ERROR_EXTEND = TICK_5_ERROR_EXTEND
		TICK_ERROR = TICK_5_ERROR
	} else {
		TICK_ERROR_EXTEND = TICK_10_ERROR_EXTEND
		TICK_ERROR = TICK_10_ERROR
	}
}

type Timeout struct {
	session       *Session
	start         time.Time
	timer         *time.Timer
	lastExtension int64
	log           logger.ILogger
	active        int32
	disabled      int32
	reset         chan int64
	c             chan time.Time
	timeout       bool
	hasReset      bool
}

func NewTimeout(s *Session, d time.Duration) *Timeout {
	t := &Timeout{
		session: s,
		timer: time.NewTimer(d),
		lastExtension: TICK_ERROR,
		log: logger.NilLogger,
		reset: make(chan int64, 1),
		c: make(chan time.Time, 1),
	}
	go t.validateTimeout(s.done)
	return t
}

func (t *Timeout) Start() time.Time {
	return t.StartWithCalibration(time.Now())
}

func (t *Timeout) StartWithCalibration(start time.Time) time.Time {
	t.start = start
	return t.start
}

func (t *Timeout) Since() time.Duration {
	return time.Since(t.start)
}

func (t *Timeout) C() <-chan time.Time {
	return t.c
}

func (t *Timeout) Stop() {
	// Drain the timer to be accurate and safe to reset.
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}
}

func (t *Timeout) Halt() {
	// Prevent timeout being triggered or reset after stopped
	t.Disable()

	t.Stop()
}

func (t *Timeout) Restart(ext int64) {
	// Prevent timeout being triggered or reset after stopped
	t.Enable()

	t.ResetWithExtension(ext)
}

func (t *Timeout) Reset() bool {
	t.session.Lock()
	defer t.session.Unlock()

	if t.timeout || t.IsDisabled() {
		return false
	}

	if t.session.isDoneLocked() {
		return false
	}

	t.resetLocked()
	return true
}

func (t *Timeout) ResetWithExtension(ext int64) bool {
	t.lastExtension = ext
	return t.Reset()
}

func (t *Timeout) SetLogger(log logger.ILogger) {
	t.log = log
}

func (t *Timeout) Busy() {
	atomic.AddInt32(&t.active, 1)
}

func (t *Timeout) DoneBusy() {
	atomic.AddInt32(&t.active, -1)
}

func (t *Timeout) DoneBusyWithReset(ext int64) {
	t.ResetWithExtension(ext)
	atomic.AddInt32(&t.active, -1)
}

func (t *Timeout) IsBusy() bool {
	return atomic.LoadInt32(&t.active) > 0
}

// Disable timeout and returns false if timeout has been disabled already
func (t *Timeout) Disable() bool {
	return atomic.CompareAndSwapInt32(&t.disabled, 0, 1)
}

// Enable timeout and returns false if timeout has been enabled already
func (t *Timeout) Enable() bool {
	return atomic.CompareAndSwapInt32(&t.disabled, 1, 0)
}

func (t *Timeout) IsDisabled() bool {
	return atomic.LoadInt32(&t.disabled) > 0
}

func (t *Timeout) validateTimeout(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case extension := <-t.reset:
			t.Stop()
			timeout := t.getTimeout(extension)
			t.timer.Reset(timeout)
			t.log.Debug("Timeout reset: %v", timeout)
			t.hasReset = false
		case ti := <-t.timer.C:
			// Timeout channel should be empty, or we clear it
			select{
			case <-t.c:
			default:
				// Nothing
			}

			t.session.Lock()
			// Double check timeout after locked.
			if t.hasReset || t.IsDisabled() {
				// pass
			} else if t.IsBusy() {
				t.resetLocked()
			} else {
				t.c <- ti
				t.timeout = true
			}
			t.session.Unlock()
		}
	}
}

func (t *Timeout) resetLocked() {
	select{
	case t.reset <- t.lastExtension:
	default:
		// Consume unread and replace with latest.
		<-t.reset
		t.reset <- t.lastExtension
	}
	t.hasReset = true
}

func (t *Timeout) getTimeout(ext int64) time.Duration {
	if ext < 0 {
		return 1 * time.Millisecond
	}

	now := time.Now().Sub(t.start).Nanoseconds()
	return time.Duration(int64(math.Ceil(float64(now + ext) / float64(TICK)))*TICK - TICK_ERROR - now)
}
