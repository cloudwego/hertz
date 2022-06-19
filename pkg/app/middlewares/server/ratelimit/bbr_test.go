package ratelimit

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	windowSizeTest   = time.Second
	bucketNumTest    = 10
	cpuThresholdTest = int64(800)

	optsForTest = []Option{
		WithWindow(windowSizeTest),
		WithBucket(bucketNumTest),
		WithCPUThreshold(cpuThresholdTest),
	}
)

func warmup(bbr *BBR, count int) {
	for i := 0; i < count; i++ {
		done, err := bbr.Allow()
		time.Sleep(time.Millisecond * 1)
		if err == nil {
			done()
		}
	}
}

func forceAllow(bbr *BBR) {
	inflight := bbr.inFlight
	bbr.inFlight = bbr.maxPASS() - 1
	done, err := bbr.Allow()
	if err == nil {
		done()
	}
	bbr.inFlight = inflight
}

func TestBBR(t *testing.T) {
	limiter := NewLimiter(
		WithWindow(5*time.Second),
		WithBucket(50),
		WithCPUThreshold(100))
	var wg sync.WaitGroup
	var drop int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				done, err := limiter.Allow()
				if err != nil {
					atomic.AddInt64(&drop, 1)
				} else {
					count := rand.Intn(100)
					time.Sleep(time.Millisecond * time.Duration(count))
					done()
				}
			}
		}()
	}
	wg.Wait()
	t.Log("drop: ", drop)
}

func TestBBRMaxPass(t *testing.T) {
	bucketDuration := windowSizeTest / time.Duration(bucketNumTest)
	bbr := NewLimiter(optsForTest...)
	for i := 1; i <= 10; i++ {
		bbr.passStat.Add(float64(i * 100))
		time.Sleep(bucketDuration)
	}
	assert.Equal(t, int64(1000), bbr.maxPASS())
	// // default max pass is equal to 1.
	bbr = NewLimiter(optsForTest...)
	assert.Equal(t, int64(1), bbr.maxPASS())
}

func TestBBRMaxPassWithCache(t *testing.T) {
	bucketDuration := windowSizeTest / time.Duration(bucketNumTest)
	bbr := NewLimiter(optsForTest...)
	// witch cache, value of latest bucket is not counted instantly.
	// after a bucket duration time, this bucket will be fully counted.
	bbr.passStat.Add(float64(50))
	time.Sleep(bucketDuration / 2)
	assert.Equal(t, int64(1), bbr.maxPASS())

	bbr.passStat.Add(float64(50))
	time.Sleep(bucketDuration / 2)
	assert.Equal(t, int64(1), bbr.maxPASS())

	bbr.passStat.Add(float64(1))
	time.Sleep(bucketDuration)
	assert.Equal(t, int64(100), bbr.maxPASS())
}

func TestBBRMinRt(t *testing.T) {
	bucketDuration := windowSizeTest / time.Duration(bucketNumTest)
	bbr := NewLimiter(optsForTest...)
	for i := 0; i < 10; i++ {
		for j := i*10 + 1; j <= i*10+10; j++ {
			bbr.rtStat.Add(float64(j))
		}
		if i != 9 {
			time.Sleep(bucketDuration)
		}
	}
	assert.Equal(t, int64(6), bbr.minRT())

	// default max min rt is equal to maxFloat64.
	bbr = NewLimiter(optsForTest...)
	bbr.rtStat = NewRollingWindow(10, bucketDuration)
	assert.Equal(t, int64(1), bbr.minRT())
}

func TestBBRMinRtWithCache(t *testing.T) {
	bucketDuration := windowSizeTest / time.Duration(bucketNumTest)
	bbr := NewLimiter(optsForTest...)
	for i := 0; i < 10; i++ {
		for j := i*10 + 1; j <= i*10+5; j++ {
			bbr.rtStat.Add(float64(j))
		}
		if i != 9 {
			time.Sleep(bucketDuration / 2)
		}
		_ = bbr.minRT()
		for j := i*10 + 6; j <= i*10+10; j++ {
			bbr.rtStat.Add(float64(j))
		}
		if i != 9 {
			time.Sleep(bucketDuration / 2)
		}
	}
	assert.Equal(t, int64(6), bbr.minRT())
}

func TestBBRMaxQps(t *testing.T) {
	bbr := NewLimiter(optsForTest...)
	bucketDuration := windowSizeTest / time.Duration(bucketNumTest)
	passStat := NewRollingWindow(10, bucketDuration)
	rtStat := NewRollingWindow(10, bucketDuration)
	for i := 0; i < 10; i++ {
		passStat.Add(float64((i + 1) * 100))
		for j := i*10 + 1; j <= i*10+10; j++ {
			rtStat.Add(float64(j))
		}
		if i != 9 {
			time.Sleep(bucketDuration)
		}
	}
	bbr.passStat = passStat
	bbr.rtStat = rtStat
	assert.Equal(t, int64(60), bbr.maxInFlight())
}

func TestBBRShouldDrop(t *testing.T) {
	var cpu int64
	bbr := NewLimiter(optsForTest...)
	bbr.cpu = func() int64 {
		return cpu
	}
	bucketDuration := windowSizeTest / time.Duration(bucketNumTest)
	passStat := NewRollingWindow(10, bucketDuration)
	rtStat := NewRollingWindow(10, bucketDuration)
	for i := 0; i < 10; i++ {
		passStat.Add(float64((i + 1) * 100))
		for j := i*10 + 1; j <= i*10+10; j++ {
			rtStat.Add(float64(j))
		}
		if i != 9 {
			time.Sleep(bucketDuration)
		}
	}
	bbr.passStat = passStat
	bbr.rtStat = rtStat
	// cpu >=  800, inflight < maxQps
	cpu = 800
	bbr.inFlight = 50
	assert.Equal(t, false, bbr.shouldDrop())

	// cpu >=  800, inflight > maxQps
	cpu = 800
	bbr.inFlight = 80
	assert.Equal(t, true, bbr.shouldDrop())

	// cpu < 800, inflight > maxQps, cold duration
	cpu = 700
	bbr.inFlight = 80
	assert.Equal(t, true, bbr.shouldDrop())

	// cpu < 800, inflight > maxQps
	time.Sleep(2 * time.Second)
	cpu = 700
	bbr.inFlight = 80
	assert.Equal(t, false, bbr.shouldDrop())
}

func BenchmarkBBRAllowUnderLowLoad(b *testing.B) {
	bbr := NewLimiter(optsForTest...)
	bbr.cpu = func() int64 {
		return 500
	}
	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		done, err := bbr.Allow()
		if err == nil {
			done()
		}
	}
}

func BenchmarkBBRAllowUnderHighLoad(b *testing.B) {
	bbr := NewLimiter(optsForTest...)
	bbr.cpu = func() int64 {
		return 900
	}
	bbr.inFlight = 1
	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		if i%10000 == 0 {
			maxFlight := bbr.maxInFlight()
			if maxFlight != 0 {
				bbr.inFlight = rand.Int63n(bbr.maxInFlight() * 2)
			}
		}
		done, err := bbr.Allow()
		if err == nil {
			done()
		}
	}
}

func BenchmarkBBRShouldDropUnderLowLoad(b *testing.B) {
	bbr := NewLimiter(optsForTest...)
	bbr.cpu = func() int64 {
		return 500
	}
	warmup(bbr, 10000)
	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		bbr.shouldDrop()
	}
}

func BenchmarkBBRShouldDropUnderHighLoad(b *testing.B) {
	bbr := NewLimiter(optsForTest...)
	bbr.cpu = func() int64 {
		return 900
	}
	warmup(bbr, 10000)
	bbr.inFlight = 1000
	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		bbr.shouldDrop()
		if i%10000 == 0 {
			forceAllow(bbr)
		}
	}
}

func BenchmarkBBRShouldDropUnderUnstableLoad(b *testing.B) {
	bbr := NewLimiter(optsForTest...)
	bbr.cpu = func() int64 {
		return 500
	}
	warmup(bbr, 10000)
	bbr.prevDropTime.Store(time.Now().UnixNano())
	bbr.inFlight = 1000
	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		bbr.shouldDrop()
		if i%100000 == 0 {
			forceAllow(bbr)
		}
	}
}
