package ratelimit

import "time"

type Option func(o *options)

//default option
var opt = options{
	Window:       time.Second * 10,
	Bucket:       100,                    // 100ms
	CPUThreshold: 800,                    // CPU load  80%
	SamplingTime: 500 * time.Millisecond, //
	Decay:        0.95,                   //
}

// options of bbr limiter.
type options struct {
	// WindowSize defines time duration per window
	Window time.Duration
	// BucketNum defines bucket number for each window
	Bucket int
	// CPUThreshold
	CPUThreshold int64
	SamplingTime time.Duration
	Decay        float64
}

//  window size.
func WithWindow(window time.Duration) Option {
	return func(o *options) {
		o.Window = window
	}
}

// bucket ize.
func WithBucket(bucket int) Option {
	return func(o *options) {
		o.Bucket = bucket
	}
}

// cpu threshold
func WithCPUThreshold(threshold int64) Option {
	return func(o *options) {
		o.CPUThreshold = threshold
	}
}

// sapmleing time
func WithSamplingTime(samplingTime time.Duration) Option {
	return func(o *options) {
		o.SamplingTime = samplingTime
	}
}

// decay time
func WithDecay(decay float64) Option {
	return func(o *options) {
		o.Decay = decay
	}
}

func NewOption(opts ...Option) options {
	for _, apply := range opts {
		apply(&opt)
	}
	return opt
}
