package congestion

// WindowedFilter Use the following to construct a windowed filter object of type T.
// For example, a min filter using QuicTime as the time type:
//
//	WindowedFilter<T, MinFilter<T>, QuicTime, QuicTime::Delta> ObjectName;
//
// A max filter using 64-bit integers as the time type:
//
//	WindowedFilter<T, MaxFilter<T>, uint64_t, int64_t> ObjectName;
//
// Specifically, this template takes four arguments:
//  1. T -- type of the measurement that is being filtered.
//  2. Compare -- MinFilter<T> or MaxFilter<T>, depending on the type of filter
//     desired.
//  3. TimeT -- the type used to represent timestamps.
//  4. TimeDeltaT -- the type used to represent continuous time intervals between
//     two timestamps.  Has to be the type of (a - b) if both |a| and |b| are
//     of type TimeT.
type WindowedFilter struct {
	// Time length of window.
	windowLength int64
	estimates    []Sample
	comparator   func(int64, int64) bool
}

type Sample struct {
	sample int64
	time   int64
}

// Compares two values and returns true if the first is greater than or equal
// to the second.
func MaxFilter(a, b int64) bool {
	return a >= b
}

// Compares two values and returns true if the first is less than or equal
// to the second.
func MinFilter(a, b int64) bool {
	return a <= b
}

func NewWindowedFilter(windowLength int64, comparator func(int64, int64) bool) *WindowedFilter {
	return &WindowedFilter{
		windowLength: windowLength,
		estimates:    make([]Sample, 3),
		comparator:   comparator,
	}
}

// Changes the window length.  Does not update any current samples.
func (f *WindowedFilter) SetWindowLength(windowLength int64) {
	f.windowLength = windowLength
}

func (f *WindowedFilter) GetBest() int64 {
	return f.estimates[0].sample
}

func (f *WindowedFilter) GetSecondBest() int64 {
	return f.estimates[1].sample
}

func (f *WindowedFilter) GetThirdBest() int64 {
	return f.estimates[2].sample
}

func (f *WindowedFilter) Update(sample int64, time int64) {
	if f.estimates[0].time == 0 || f.comparator(sample, f.estimates[0].sample) || (time-f.estimates[2].time) > f.windowLength {
		f.Reset(sample, time)
		return
	}

	if f.comparator(sample, f.estimates[1].sample) {
		f.estimates[1].sample = sample
		f.estimates[1].time = time
		f.estimates[2].sample = sample
		f.estimates[2].time = time
	} else if f.comparator(sample, f.estimates[2].sample) {
		f.estimates[2].sample = sample
		f.estimates[2].time = time
	}

	// Expire and update estimates as necessary.
	if time-f.estimates[0].time > f.windowLength {
		// The best estimate hasn't been updated for an entire window, so promote
		// second and third best estimates.
		f.estimates[0].sample = f.estimates[1].sample
		f.estimates[0].time = f.estimates[1].time
		f.estimates[1].sample = f.estimates[2].sample
		f.estimates[1].time = f.estimates[2].time
		f.estimates[2].sample = sample
		f.estimates[2].time = time
		// Need to iterate one more time. Check if the new best estimate is
		// outside the window as well, since it may also have been recorded a
		// long time ago. Don't need to iterate once more since we cover that
		// case at the beginning of the method.
		if time-f.estimates[0].time > f.windowLength {
			f.estimates[0].sample = f.estimates[1].sample
			f.estimates[0].time = f.estimates[1].time
			f.estimates[1].sample = f.estimates[2].sample
			f.estimates[1].time = f.estimates[2].time
		}
		return
	}
	if f.estimates[1].sample == f.estimates[0].sample && time-f.estimates[1].time > f.windowLength>>2 {
		// A quarter of the window has passed without a better sample, so the
		// second-best estimate is taken from the second quarter of the window.
		f.estimates[1].sample = sample
		f.estimates[1].time = time
		f.estimates[2].sample = sample
		f.estimates[2].time = time
		return
	}

	if f.estimates[2].sample == f.estimates[1].sample && time-f.estimates[2].time > f.windowLength>>1 {
		// We've passed a half of the window without a better estimate, so take
		// a third-best estimate from the second half of the window.
		f.estimates[2].sample = sample
		f.estimates[2].time = time
	}
}

func (f *WindowedFilter) Reset(newSample int64, newTime int64) {
	f.estimates[0].sample = newSample
	f.estimates[0].time = newTime
	f.estimates[1].sample = newSample
	f.estimates[1].time = newTime
	f.estimates[2].sample = newSample
	f.estimates[2].time = newTime
}
