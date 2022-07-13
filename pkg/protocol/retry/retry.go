package retry

import (
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"io"
	"math"
	"math/rand"
	"time"
)

// RetryConfig All configurations related to retry
type RetryConfig struct {

	//consts.DefaultMaxIdempotentCallAttempts 最大重试次数
	MaxIdempotentCallAttempts uint

	// 重试延迟时间
	Delay time.Duration

	// 最大重试延迟时间，选择指数退避策略时，该配置会限制等待时间上限
	MaxDelay time.Duration

	// 最大抖动时间
	MaxJitter time.Duration

	//该字段待定，每次重试时进行的一次回调
	//onRetry RetryFunc

	// 发生错误时判断是否满足重试条件,当前默认 isIdempotent ，用户可指定
	RetryIf RetryIfFunc

	// CombineDelay(BackOffDelay, RandomDelay)退避策略，将多种退避策略结合在一起
	DelayPolicy DelayPolicyFunc
}

func NewDefaultRetryConfig(retryCfg *RetryConfig) *RetryConfig {
	// 默认关闭重试
	if retryCfg == nil {
		return &RetryConfig{MaxIdempotentCallAttempts: consts.DefaultMaxIdempotentCallAttempts}
	}

	cfg := &RetryConfig{
		// 后续默认值都写在一起，这里先放着
		MaxIdempotentCallAttempts: consts.DefaultMaxIdempotentCallAttempts,
		Delay:                     1 * time.Millisecond,
		MaxDelay:                  100 * time.Millisecond,
		MaxJitter:                 10 * time.Millisecond,
		RetryIf:                   DefaultRetryIf,
		DelayPolicy:               CombineDelay(DefaultDelay),
	}
	// 不太优雅，感觉可以用 选项模式 withXXX代替，不过这样可能增加用户操作成本
	if retryCfg.MaxIdempotentCallAttempts != 0 {
		cfg.MaxIdempotentCallAttempts = retryCfg.MaxIdempotentCallAttempts
	}
	if retryCfg.Delay != 0 {
		cfg.Delay = retryCfg.Delay
	}
	if retryCfg.MaxDelay != 0 {
		cfg.MaxDelay = retryCfg.MaxDelay
	}
	if retryCfg.MaxJitter != 0 {
		cfg.MaxJitter = retryCfg.MaxJitter
	}
	if retryCfg.RetryIf != nil {
		cfg.RetryIf = retryCfg.RetryIf
	}
	if retryCfg.DelayPolicy != nil {
		cfg.DelayPolicy = retryCfg.DelayPolicy
	}
	return cfg
}

// RetryIfFunc signature of retry if function
//
// Request argument passed to RetryIfFunc, if there are any request errors.
type RetryIfFunc func(request *protocol.Request, response *protocol.Response, err error) bool

// DelayPolicyFunc is called to return the next delay to wait after the retriable function fails on `err` after attempts.
type DelayPolicyFunc func(attempts uint, err error, retryConfig *RetryConfig) time.Duration

func DefaultRetryIf(req *protocol.Request, resp *protocol.Response, err error) bool {

	if req.IsBodyStream() {
		return false
	}
	// Retry non-idempotent requests if the server closes
	// the connection before sending the response.
	//
	// This case is possible if the server closes the idle
	// keep-alive connection on timeout.
	//
	// Apache and nginx usually do this.
	if !isIdempotent(req, resp, err) && err == io.EOF {
		return true
	}
	if resp.StatusCode() == 0 || resp.StatusCode() == 429 || (resp.StatusCode()) >= 500 && resp.StatusCode() != 501 {
		return true
	}
	return false
}

func isIdempotent(req *protocol.Request, response *protocol.Response, err error) bool {
	return req.Header.IsGet() || req.Header.IsHead() || req.Header.IsPut() || req.Header.IsDelete()
}

// DefaultDelay is a DelayPolicyFunc which keep 0 delay in all iterations
func DefaultDelay(attempts uint, err error, retryConfig *RetryConfig) time.Duration {
	return 0 * time.Millisecond
}

// FixedDelay is a DelayPolicyFunc which keeps delay the same through all iterations
func FixedDelay(_ uint, _ error, retryConfig *RetryConfig) time.Duration {
	return retryConfig.Delay
}

// RandomDelay is a DelayPolicyFunc which picks a random delay up to RetryConfig.maxJitter
func RandomDelay(_ uint, _ error, retryConfig *RetryConfig) time.Duration {
	return time.Duration(rand.Int63n(int64(retryConfig.MaxJitter)))
}

// BackOffDelay is a DelayPolicyFunc which increases delay between consecutive retries
func BackOffDelay(attempts uint, _ error, retryConfig *RetryConfig) time.Duration {
	// 1 << 63 would overflow signed int64 (time.Duration), thus 62.
	const max uint = 62

	//if retryConfig.MaxBackOffN == 0 {
	//	if retryConfig.Delay <= 0 {
	//		retryConfig.Delay = 1
	//	}
	//
	//	retryConfig.MaxBackOffN = max - uint(math.Floor(math.Log2(float64(retryConfig.Delay))))
	//}

	if attempts > max {
		attempts = max
	}

	return retryConfig.Delay << attempts
}

// CombineDelay is a DelayPolicy to combines all of the specified delays into a new DelayPolicyFunc
// 将前面三种策略混合在一起共用
func CombineDelay(delays ...DelayPolicyFunc) DelayPolicyFunc {
	const maxInt64 = uint64(math.MaxInt64)

	return func(attempts uint, err error, config *RetryConfig) time.Duration {
		var total uint64
		for _, delay := range delays {
			total += uint64(delay(attempts, err, config))
			if total > maxInt64 {
				total = maxInt64
			}
		}

		return time.Duration(total)
	}
}

func Delay(attempts uint, err error, retryConfig *RetryConfig) time.Duration {
	delayTime := retryConfig.DelayPolicy(attempts, err, retryConfig)
	if retryConfig.MaxDelay > 0 && delayTime > retryConfig.MaxDelay {
		delayTime = retryConfig.MaxDelay
	}

	return delayTime
}
