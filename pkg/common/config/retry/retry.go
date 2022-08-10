/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package retry

import (
	"io"
	"math"
	"math/rand"
	"time"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// RetryConfig All configurations related to retry
type RetryConfig struct {
	// The maximum number of idempotent call retries, including the initial call
	MaxIdempotentCallAttempts uint

	// Initial retry delay time
	Delay time.Duration

	// Maximum retry delay time. When the retry time increases beyond this time,
	// this configuration will limit the upper limit of waiting time
	MaxDelay time.Duration

	// The maximum jitter time, which takes effect when the delay policy is configured as RandomDelay
	MaxJitter time.Duration

	// This field is pending. A callback at each retry
	// retryCallback RetryFunc

	// Judge whether the retry condition is met when an error occurs,which can be customized by the user
	RetryIf RetryIfFunc

	// Delay strategy, which can combine multiple delay strategies. such as CombineDelay(BackOffDelay, RandomDelay) or BackOffDelay,etc
	DelayPolicy DelayPolicyFunc
}

func (o *RetryConfig) Apply(opts []RetryOption) {
	for _, op := range opts {
		op.F(o)
	}
}

func NewRetryConfig(opts ...RetryOption) (*RetryConfig, error) {
	retryCfg := &RetryConfig{
		MaxIdempotentCallAttempts: consts.DefaultMaxIdempotentCallAttempts,
		Delay:                     1 * time.Millisecond,
		MaxDelay:                  100 * time.Millisecond,
		MaxJitter:                 20 * time.Millisecond,
		RetryIf:                   DefaultRetryIf,
		DelayPolicy:               CombineDelay(DefaultDelay),
	}
	retryCfg.Apply(opts)

	return retryCfg, nil
}

// RetryIfFunc signature of retry if function
// Judge whether to retry by request,response or error
type RetryIfFunc func(req *protocol.Request, resp *protocol.Response, err error) bool

// DelayPolicyFunc signature of delay policy function
// is called to return the delay of retry
type DelayPolicyFunc func(attempts uint, err error, retryConfig *RetryConfig) time.Duration

// DefaultRetryIf Default retry condition, to be optimized
func DefaultRetryIf(req *protocol.Request, resp *protocol.Response, err error) bool {
	if !req.IsBodyStream() {
		return true
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
	// Retry the response in the range of 500,
	// because 5xx is usually not a permanent error,
	// which may be related to the interruption and downtime of the server.
	// Give the server a certain backoff time to recover.
	// This will also catch invalid response codes, such as 0, etc
	if resp.StatusCode() == 0 || (resp.StatusCode()) >= 500 && resp.StatusCode() != 501 {
		return true
	}
	return false
}

func isIdempotent(req *protocol.Request, resp *protocol.Response, err error) bool {
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

// RandomDelay is a DelayPolicyFunc which picks a random delay up to RetryConfig.MaxJitter
func RandomDelay(_ uint, _ error, retryConfig *RetryConfig) time.Duration {
	return time.Duration(rand.Int63n(int64(retryConfig.MaxJitter)))
}

// BackOffDelay is a DelayPolicyFunc which exponentially increases delay between consecutive retries
func BackOffDelay(attempts uint, _ error, retryConfig *RetryConfig) time.Duration {
	// 1 << 63 would overflow signed int64 (time.Duration), thus 62.
	const max uint = 62
	if attempts > max {
		attempts = max
	}

	return retryConfig.Delay << attempts
}

// CombineDelay return DelayPolicyFunc, which combines the optional DelayPolicyFunc into a new DelayPolicyFunc
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

// Delay generate the delay time required for the current retry
func Delay(attempts uint, err error, retryConfig *RetryConfig) time.Duration {
	delayTime := retryConfig.DelayPolicy(attempts, err, retryConfig)
	if retryConfig.MaxDelay > 0 && delayTime > retryConfig.MaxDelay {
		delayTime = retryConfig.MaxDelay
	}
	return delayTime
}
