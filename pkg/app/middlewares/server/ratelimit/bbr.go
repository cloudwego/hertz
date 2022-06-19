package ratelimit

import (
	"errors"
	"math"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

var (
	gCPU int64
)

type (
	cpuGetter func() int64
)

func init() {
	go cpuproc()
}

// cpu = cpuᵗ⁻¹ * decay + cpuᵗ * (1 - decay)
// EMA Fix CPU load situation
func cpuproc() {
	ticker := time.NewTicker(opt.SamplingTime) // same to cpu sample rate
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go cpuproc()
		}
	}()

	// EMA algorithm: https://blog.csdn.net/m0_38106113/article/details/81542863
	for range ticker.C {
		usage, _ := cpu.Percent(opt.SamplingTime, false)
		prevCPU := atomic.LoadInt64(&gCPU)
		curCPU := int64(float64(prevCPU)*opt.Decay + float64(usage[0]*10)*(1.0-opt.Decay))
		atomic.StoreInt64(&gCPU, curCPU)
	}
}

// counterCache is used to cache maxPASS and minRt result.
// Value of current bucket is not counted in real time.
// Cache time is equal to a bucket duration.
type counterCache struct {
	val  int64
	time time.Time
}

// BBR implements bbr-like limiter.
// It is inspired by sentinel.
// https://github.com/alibaba/Sentinel/wiki/%E7%B3%BB%E7%BB%9F%E8%87%AA%E9%80%82%E5%BA%94%E9%99%90%E6%B5%81
type BBR struct {
	cpu             cpuGetter
	passStat        *RollingWindow // request succeeded
	rtStat          *RollingWindow // time consume
	inFlight        int64          // Number of requests being processed
	bucketPerSecond int64
	bucketDuration  time.Duration

	// prevDropTime defines previous start drop since initTime
	prevDropTime atomic.Value
	maxPASSCache atomic.Value
	minRtCache   atomic.Value

	opts options
}

// NewLimiter returns a bbr limiter
func NewLimiter(opts ...Option) *BBR {
	opt := NewOption(opts...)
	bucketDuration := opt.Window / time.Duration(opt.Bucket)
	// 10s / 100  = 100ms
	passStat := NewRollingWindow(opt.Bucket, bucketDuration, IgnoreCurrentBucket())
	rtStat := NewRollingWindow(opt.Bucket, bucketDuration, IgnoreCurrentBucket())

	limiter := &BBR{
		opts:            opt,
		passStat:        passStat,
		rtStat:          rtStat,
		bucketDuration:  bucketDuration,
		bucketPerSecond: int64(time.Second / bucketDuration),
		cpu:             func() int64 { return atomic.LoadInt64(&gCPU) },
	}

	return limiter
}

//Maximum number of requests in a single sampling window
func (l *BBR) maxPASS() int64 {
	passCache := l.maxPASSCache.Load()
	if passCache != nil {
		ps := passCache.(*counterCache)
		if l.timespan(ps.time) < 1 {
			return ps.val
		}
		//Avoid glitches caused by fluctuations
	}
	var rawMaxPass float64
	l.passStat.Reduce(func(b *Bucket) {
		rawMaxPass = math.Max(float64(b.Sum), rawMaxPass)
	})
	if rawMaxPass <= 0 {
		rawMaxPass = 1
	}
	l.maxPASSCache.Store(&counterCache{
		val:  int64(rawMaxPass),
		time: time.Now(),
	})
	return int64(rawMaxPass)
}

// timespan returns the passed bucket count
// since lastTime, if it is one bucket duration earlier than
// the last recorded time, it will return the BucketNum.
func (l *BBR) timespan(lastTime time.Time) int {
	v := int(time.Since(lastTime) / l.bucketDuration)
	if v > -1 {
		return v
	}
	return l.opts.Bucket
}

// Minimum response time
func (l *BBR) minRT() int64 {
	rtCache := l.minRtCache.Load()
	if rtCache != nil {
		rc := rtCache.(*counterCache)
		if l.timespan(rc.time) < 1 {
			return rc.val
		}
	}
	// Go to the nearest response time within 1s
	var rawMinRT float64 = 1 << 31
	l.rtStat.Reduce(func(b *Bucket) {
		if b.Count <= 0 {
			return
		}
		if rawMinRT > math.Ceil(b.Sum/float64(b.Count)) {
			rawMinRT = math.Ceil(b.Sum / float64(b.Count))
		}
	})
	if rawMinRT == 1<<31 {
		rawMinRT = 1
	}
	l.minRtCache.Store(&counterCache{
		val:  int64(rawMinRT),
		time: time.Now(),
	})
	return int64(rawMinRT)
}

func (l *BBR) maxInFlight() int64 {
	return int64(math.Ceil(float64(l.maxPASS()*l.minRT()*l.bucketPerSecond) / 1000.0))
}

func (l *BBR) shouldDrop() bool {
	now := time.Duration(time.Now().UnixNano())
	if l.cpu() < l.opts.CPUThreshold {
		// current cpu payload below the threshold
		prevDropTime, _ := l.prevDropTime.Load().(time.Duration)
		if prevDropTime == 0 {
			// haven't start drop,
			// accept current request
			return false
		}
		if time.Duration(now-prevDropTime) <= time.Second {
			// just start drop one second ago,
			// check current inflight count
			inFlight := atomic.LoadInt64(&l.inFlight)
			return inFlight > 1 && inFlight > l.maxInFlight()
		}
		l.prevDropTime.Store(time.Duration(0))
		return false
	}
	// current cpu payload exceeds the threshold
	inFlight := atomic.LoadInt64(&l.inFlight)
	drop := inFlight > 1 && inFlight > l.maxInFlight()
	if drop {
		prevDrop, _ := l.prevDropTime.Load().(time.Duration)
		if prevDrop != 0 {
			// already started drop, return directly
			return drop
		}
		// store start drop time
		l.prevDropTime.Store(now)
	}
	return drop
}

// Allow checks all inbound traffic.

func (l *BBR) Allow() (func(), error) {
	if l.shouldDrop() {
		return nil, errors.New(ErrLimit)
	}
	atomic.AddInt64(&l.inFlight, 1)
	start := time.Now().UnixNano()
	//DoneFunc record time-consuming
	return func() {
		rt := (time.Now().UnixNano() - start) / int64(time.Millisecond)
		l.rtStat.Add(float64(rt))
		atomic.AddInt64(&l.inFlight, -1)
		l.passStat.Add(1)
	}, nil
}
