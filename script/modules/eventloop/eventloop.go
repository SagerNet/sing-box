package eventloop

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

type job struct {
	cancel func() bool
	fn     func()
	idx    int

	cancelled bool
}

type Timer struct {
	job
	timer *time.Timer
}

type Interval struct {
	job
	ticker   *time.Ticker
	stopChan chan struct{}
}

type Immediate struct {
	job
}

type EventLoop struct {
	vm       *goja.Runtime
	jobChan  chan func()
	jobs     []*job
	jobCount int32
	canRun   int32

	auxJobsLock sync.Mutex
	wakeupChan  chan struct{}

	auxJobsSpare, auxJobs []func()

	stopLock   sync.Mutex
	stopCond   *sync.Cond
	running    bool
	terminated bool

	errorHandler func(error)
}

func Enable(runtime *goja.Runtime, errorHandler func(error)) *EventLoop {
	loop := &EventLoop{
		vm:           runtime,
		jobChan:      make(chan func()),
		wakeupChan:   make(chan struct{}, 1),
		errorHandler: errorHandler,
	}
	loop.stopCond = sync.NewCond(&loop.stopLock)
	runtime.Set("setTimeout", loop.setTimeout)
	runtime.Set("setInterval", loop.setInterval)
	runtime.Set("setImmediate", loop.setImmediate)
	runtime.Set("clearTimeout", loop.clearTimeout)
	runtime.Set("clearInterval", loop.clearInterval)
	runtime.Set("clearImmediate", loop.clearImmediate)
	return loop
}

func (loop *EventLoop) schedule(call goja.FunctionCall, repeating bool) goja.Value {
	if fn, ok := goja.AssertFunction(call.Argument(0)); ok {
		delay := call.Argument(1).ToInteger()
		var args []goja.Value
		if len(call.Arguments) > 2 {
			args = append(args, call.Arguments[2:]...)
		}
		f := func() {
			_, err := fn(nil, args...)
			if err != nil {
				loop.errorHandler(err)
			}
		}
		loop.jobCount++
		var job *job
		var ret goja.Value
		if repeating {
			interval := loop.newInterval(f)
			interval.start(loop, time.Duration(delay)*time.Millisecond)
			job = &interval.job
			ret = loop.vm.ToValue(interval)
		} else {
			timeout := loop.newTimeout(f)
			timeout.start(loop, time.Duration(delay)*time.Millisecond)
			job = &timeout.job
			ret = loop.vm.ToValue(timeout)
		}
		job.idx = len(loop.jobs)
		loop.jobs = append(loop.jobs, job)
		return ret
	}
	return nil
}

func (loop *EventLoop) setTimeout(call goja.FunctionCall) goja.Value {
	return loop.schedule(call, false)
}

func (loop *EventLoop) setInterval(call goja.FunctionCall) goja.Value {
	return loop.schedule(call, true)
}

func (loop *EventLoop) setImmediate(call goja.FunctionCall) goja.Value {
	if fn, ok := goja.AssertFunction(call.Argument(0)); ok {
		var args []goja.Value
		if len(call.Arguments) > 1 {
			args = append(args, call.Arguments[1:]...)
		}
		f := func() {
			_, err := fn(nil, args...)
			if err != nil {
				loop.errorHandler(err)
			}
		}
		loop.jobCount++
		return loop.vm.ToValue(loop.addImmediate(f))
	}
	return nil
}

// SetTimeout schedules to run the specified function in the context
// of the loop as soon as possible after the specified timeout period.
// SetTimeout returns a Timer which can be passed to ClearTimeout.
// The instance of goja.Runtime that is passed to the function and any Values derived
// from it must not be used outside the function. SetTimeout is
// safe to call inside or outside the loop.
// If the loop is terminated (see Terminate()) returns nil.
func (loop *EventLoop) SetTimeout(fn func(*goja.Runtime), timeout time.Duration) *Timer {
	t := loop.newTimeout(func() { fn(loop.vm) })
	if loop.addAuxJob(func() {
		t.start(loop, timeout)
		loop.jobCount++
		t.idx = len(loop.jobs)
		loop.jobs = append(loop.jobs, &t.job)
	}) {
		return t
	}
	return nil
}

// ClearTimeout cancels a Timer returned by SetTimeout if it has not run yet.
// ClearTimeout is safe to call inside or outside the loop.
func (loop *EventLoop) ClearTimeout(t *Timer) {
	loop.addAuxJob(func() {
		loop.clearTimeout(t)
	})
}

// SetInterval schedules to repeatedly run the specified function in
// the context of the loop as soon as possible after every specified
// timeout period.  SetInterval returns an Interval which can be
// passed to ClearInterval. The instance of goja.Runtime that is passed to the
// function and any Values derived from it must not be used outside
// the function. SetInterval is safe to call inside or outside the
// loop.
// If the loop is terminated (see Terminate()) returns nil.
func (loop *EventLoop) SetInterval(fn func(*goja.Runtime), timeout time.Duration) *Interval {
	i := loop.newInterval(func() { fn(loop.vm) })
	if loop.addAuxJob(func() {
		i.start(loop, timeout)
		loop.jobCount++
		i.idx = len(loop.jobs)
		loop.jobs = append(loop.jobs, &i.job)
	}) {
		return i
	}
	return nil
}

// ClearInterval cancels an Interval returned by SetInterval.
// ClearInterval is safe to call inside or outside the loop.
func (loop *EventLoop) ClearInterval(i *Interval) {
	loop.addAuxJob(func() {
		loop.clearInterval(i)
	})
}

func (loop *EventLoop) setRunning() {
	loop.stopLock.Lock()
	defer loop.stopLock.Unlock()
	if loop.running {
		panic("Loop is already started")
	}
	loop.running = true
	atomic.StoreInt32(&loop.canRun, 1)
	loop.auxJobsLock.Lock()
	loop.terminated = false
	loop.auxJobsLock.Unlock()
}

// Run calls the specified function, starts the event loop and waits until there are no more delayed jobs to run
// after which it stops the loop and returns.
// The instance of goja.Runtime that is passed to the function and any Values derived from it must not be used
// outside the function.
// Do NOT use this function while the loop is already running. Use RunOnLoop() instead.
// If the loop is already started it will panic.
func (loop *EventLoop) Run(fn func(*goja.Runtime)) {
	loop.setRunning()
	fn(loop.vm)
	loop.run(false)
}

// Start the event loop in the background. The loop continues to run until Stop() is called.
// If the loop is already started it will panic.
func (loop *EventLoop) Start() {
	loop.setRunning()
	go loop.run(true)
}

// StartInForeground starts the event loop in the current goroutine. The loop continues to run until Stop() is called.
// If the loop is already started it will panic.
// Use this instead of Start if you want to recover from panics that may occur while calling native Go functions from
// within setInterval and setTimeout callbacks.
func (loop *EventLoop) StartInForeground() {
	loop.setRunning()
	loop.run(true)
}

// Stop the loop that was started with Start(). After this function returns there will be no more jobs executed
// by the loop. It is possible to call Start() or Run() again after this to resume the execution.
// Note, it does not cancel active timeouts (use Terminate() instead if you want this).
// It is not allowed to run Start() (or Run()) and Stop() or Terminate() concurrently.
// Calling Stop() on a non-running loop has no effect.
// It is not allowed to call Stop() from the loop, because it is synchronous and cannot complete until the loop
// is not running any jobs. Use StopNoWait() instead.
// return number of jobs remaining
func (loop *EventLoop) Stop() int {
	loop.stopLock.Lock()
	for loop.running {
		atomic.StoreInt32(&loop.canRun, 0)
		loop.wakeup()
		loop.stopCond.Wait()
	}
	loop.stopLock.Unlock()
	return int(loop.jobCount)
}

// StopNoWait tells the loop to stop and returns immediately. Can be used inside the loop. Calling it on a
// non-running loop has no effect.
func (loop *EventLoop) StopNoWait() {
	loop.stopLock.Lock()
	if loop.running {
		atomic.StoreInt32(&loop.canRun, 0)
		loop.wakeup()
	}
	loop.stopLock.Unlock()
}

// Terminate stops the loop and clears all active timeouts and intervals. After it returns there are no
// active timers or goroutines associated with the loop. Any attempt to submit a task (by using RunOnLoop(),
// SetTimeout() or SetInterval()) will not succeed.
// After being terminated the loop can be restarted again by using Start() or Run().
// This method must not be called concurrently with Stop*(), Start(), or Run().
func (loop *EventLoop) Terminate() {
	loop.Stop()

	loop.auxJobsLock.Lock()
	loop.terminated = true
	loop.auxJobsLock.Unlock()

	loop.runAux()

	for i := 0; i < len(loop.jobs); i++ {
		job := loop.jobs[i]
		if !job.cancelled {
			job.cancelled = true
			if job.cancel() {
				loop.removeJob(job)
				i--
			}
		}
	}

	for len(loop.jobs) > 0 {
		(<-loop.jobChan)()
	}
}

// RunOnLoop schedules to run the specified function in the context of the loop as soon as possible.
// The order of the runs is preserved (i.e. the functions will be called in the same order as calls to RunOnLoop())
// The instance of goja.Runtime that is passed to the function and any Values derived from it must not be used
// outside the function. It is safe to call inside or outside the loop.
// Returns true on success or false if the loop is terminated (see Terminate()).
func (loop *EventLoop) RunOnLoop(fn func(*goja.Runtime)) bool {
	return loop.addAuxJob(func() { fn(loop.vm) })
}

func (loop *EventLoop) runAux() {
	loop.auxJobsLock.Lock()
	jobs := loop.auxJobs
	loop.auxJobs = loop.auxJobsSpare
	loop.auxJobsLock.Unlock()
	for i, job := range jobs {
		job()
		jobs[i] = nil
	}
	loop.auxJobsSpare = jobs[:0]
}

func (loop *EventLoop) run(inBackground bool) {
	loop.runAux()
	if inBackground {
		loop.jobCount++
	}
LOOP:
	for loop.jobCount > 0 {
		select {
		case job := <-loop.jobChan:
			job()
		case <-loop.wakeupChan:
			loop.runAux()
			if atomic.LoadInt32(&loop.canRun) == 0 {
				break LOOP
			}
		}
	}
	if inBackground {
		loop.jobCount--
	}

	loop.stopLock.Lock()
	loop.running = false
	loop.stopLock.Unlock()
	loop.stopCond.Broadcast()
}

func (loop *EventLoop) wakeup() {
	select {
	case loop.wakeupChan <- struct{}{}:
	default:
	}
}

func (loop *EventLoop) addAuxJob(fn func()) bool {
	loop.auxJobsLock.Lock()
	if loop.terminated {
		loop.auxJobsLock.Unlock()
		return false
	}
	loop.auxJobs = append(loop.auxJobs, fn)
	loop.auxJobsLock.Unlock()
	loop.wakeup()
	return true
}

func (loop *EventLoop) newTimeout(f func()) *Timer {
	t := &Timer{
		job: job{fn: f},
	}
	t.cancel = t.doCancel

	return t
}

func (t *Timer) start(loop *EventLoop, timeout time.Duration) {
	t.timer = time.AfterFunc(timeout, func() {
		loop.jobChan <- func() {
			loop.doTimeout(t)
		}
	})
}

func (loop *EventLoop) newInterval(f func()) *Interval {
	i := &Interval{
		job:      job{fn: f},
		stopChan: make(chan struct{}),
	}
	i.cancel = i.doCancel

	return i
}

func (i *Interval) start(loop *EventLoop, timeout time.Duration) {
	// https://nodejs.org/api/timers.html#timers_setinterval_callback_delay_args
	if timeout <= 0 {
		timeout = time.Millisecond
	}
	i.ticker = time.NewTicker(timeout)
	go i.run(loop)
}

func (loop *EventLoop) addImmediate(f func()) *Immediate {
	i := &Immediate{
		job: job{fn: f},
	}
	loop.addAuxJob(func() {
		loop.doImmediate(i)
	})
	return i
}

func (loop *EventLoop) doTimeout(t *Timer) {
	loop.removeJob(&t.job)
	if !t.cancelled {
		t.cancelled = true
		loop.jobCount--
		t.fn()
	}
}

func (loop *EventLoop) doInterval(i *Interval) {
	if !i.cancelled {
		i.fn()
	}
}

func (loop *EventLoop) doImmediate(i *Immediate) {
	if !i.cancelled {
		i.cancelled = true
		loop.jobCount--
		i.fn()
	}
}

func (loop *EventLoop) clearTimeout(t *Timer) {
	if t != nil && !t.cancelled {
		t.cancelled = true
		loop.jobCount--
		if t.doCancel() {
			loop.removeJob(&t.job)
		}
	}
}

func (loop *EventLoop) clearInterval(i *Interval) {
	if i != nil && !i.cancelled {
		i.cancelled = true
		loop.jobCount--
		i.doCancel()
	}
}

func (loop *EventLoop) removeJob(job *job) {
	idx := job.idx
	if idx < 0 {
		return
	}
	if idx < len(loop.jobs)-1 {
		loop.jobs[idx] = loop.jobs[len(loop.jobs)-1]
		loop.jobs[idx].idx = idx
	}
	loop.jobs[len(loop.jobs)-1] = nil
	loop.jobs = loop.jobs[:len(loop.jobs)-1]
	job.idx = -1
}

func (loop *EventLoop) clearImmediate(i *Immediate) {
	if i != nil && !i.cancelled {
		i.cancelled = true
		loop.jobCount--
	}
}

func (i *Interval) doCancel() bool {
	close(i.stopChan)
	return false
}

func (t *Timer) doCancel() bool {
	return t.timer.Stop()
}

func (i *Interval) run(loop *EventLoop) {
L:
	for {
		select {
		case <-i.stopChan:
			i.ticker.Stop()
			break L
		case <-i.ticker.C:
			loop.jobChan <- func() {
				loop.doInterval(i)
			}
		}
	}
	loop.jobChan <- func() {
		loop.removeJob(&i.job)
	}
}
