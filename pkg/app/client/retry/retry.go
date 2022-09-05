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
	"math"
	"math/rand"
	"time"
)

// Config All configurations related to retry
type Config struct {
	// The maximum number of call attempt times, including the initial call
	MaxAttemptTimes uint

	// Initial retry delay time
	Delay time.Duration

	// Maximum retry delay time. When the retry time increases beyond this time,
	// this configuration will limit the upper limit of waiting time
	MaxDelay time.Duration

	// The maximum jitter time, which takes effect when the delay policy is configured as RandomDelay
	MaxJitter time.Duration

	// This field is pending. A callback at each retry
	// retryCallback RetryFunc

	// Delay strategy, which can combine multiple delay strategies. such as CombineDelay(BackOffDelay, RandomDelay) or BackOffDelay,etc
	DelayPolicy DelayPolicyFunc
}

func (o *Config) Apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

// DelayPolicyFunc signature of delay policy function
// is called to return the delay of retry
type DelayPolicyFunc func(attempts uint, err error, retryConfig *Config) time.Duration

// DefaultDelayPolicy is a DelayPolicyFunc which keep 0 delay in all iterations
func DefaultDelayPolicy(attempts uint, err error, retryConfig *Config) time.Duration {
	return 0 * time.Millisecond
}

// FixedDelay is a DelayPolicyFunc which keeps delay the same through all iterations
func FixedDelay(_ uint, _ error, retryConfig *Config) time.Duration {
	return retryConfig.Delay
}

// RandomDelay is a DelayPolicyFunc which picks a random delay up toRetryConfig.MaxJitter
func RandomDelay(_ uint, _ error, retryConfig *Config) time.Duration {
	return time.Duration(rand.Int63n(int64(retryConfig.MaxJitter)))
}

// BackOffDelay is a DelayPolicyFunc which exponentially increases delay between consecutive retries
func BackOffDelay(attempts uint, _ error, retryConfig *Config) time.Duration {
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

	return func(attempts uint, err error, config *Config) time.Duration {
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
func Delay(attempts uint, err error, retryConfig *Config) time.Duration {
	delayTime := retryConfig.DelayPolicy(attempts, err, retryConfig)
	if retryConfig.MaxDelay > 0 && delayTime > retryConfig.MaxDelay {
		delayTime = retryConfig.MaxDelay
	}
	return delayTime
}
