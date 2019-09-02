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
	start         time.Time
	timer         *time.Timer
	lastExtension int64
	log           logger.ILogger
	active        int32
	disabled      int32
	c             chan time.Time
}

func NewTimeout(d time.Duration, done <-chan struct{}) *Timeout {
	t := &Timeout{
		timer: time.NewTimer(d),
		lastExtension: TICK_ERROR,
		log: logger.NilLogger,
		disabled: 1,
		c: make(chan time.Time),
	}
	go t.validateTimeout(done)
	return t
}

func (t *Timeout) Start() time.Time {
	return t.StartWithCalibration(time.Now())
}

func (t *Timeout) StartWithCalibration(start time.Time) time.Time {
	t.start = start
	// atomic.StoreInt32(&t.active, 0)
	t.Enable()
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

func (t *Timeout) Reset() {
	if !t.Disable() {
		// already disabled
		return
	}
	defer t.Enable()

	t.Stop()
	timeout := t.getTimeout(t.lastExtension)
	t.timer.Reset(timeout)
	t.log.Debug("Timeout reset: %v", timeout)
}

func (t *Timeout) ResetWithExtension(ext int64) {
	t.lastExtension = ext
	t.Reset()
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
		case ti := <-t.timer.C:
			if t.IsDisabled() {
				continue
			} else if t.IsBusy() {
				t.Reset()
			} else {
				t.c <- ti
			}
		}
	}
}

func (t *Timeout) getTimeout(ext int64) time.Duration {
	if ext < 0 {
		return 1 * time.Millisecond
	}

	now := time.Now().Sub(t.start).Nanoseconds()
	return time.Duration(int64(math.Ceil(float64(now + ext) / float64(TICK)))*TICK - TICK_ERROR - now)
}
