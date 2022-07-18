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

import "time"

// RetryOption is the only struct that can be used to set RetryConfig.
type RetryOption struct {
	F func(o *RetryConfig)
}

// WithMaxIdempotentCallAttempts sets MaxIdempotentCallAttempts.
func WithMaxIdempotentCallAttempts(maxIdempotentCallAttempts uint) RetryOption {
	return RetryOption{F: func(o *RetryConfig) {
		o.MaxIdempotentCallAttempts = maxIdempotentCallAttempts
	}}
}

// WithDelay sets MaxIdempotentCallAttempts.
func WithDelay(delay time.Duration) RetryOption {
	return RetryOption{F: func(o *RetryConfig) {
		o.Delay = delay
	}}
}

// WithMaxDelay sets MaxIdempotentCallAttempts.
func WithMaxDelay(maxDelay time.Duration) RetryOption {
	return RetryOption{F: func(o *RetryConfig) {
		o.MaxDelay = maxDelay
	}}
}

// WithRetryIf sets MaxIdempotentCallAttempts.
func WithRetryIf(retryIf RetryIfFunc) RetryOption {
	return RetryOption{F: func(o *RetryConfig) {
		o.RetryIf = retryIf
	}}
}

// WithDelayPolicy sets MaxIdempotentCallAttempts.
func WithDelayPolicy(delayPolicy DelayPolicyFunc) RetryOption {
	return RetryOption{F: func(o *RetryConfig) {
		o.DelayPolicy = delayPolicy
	}}
}

// WithMaxJitter sets MaxIdempotentCallAttempts.
func WithMaxJitter(maxJitter time.Duration) RetryOption {
	return RetryOption{F: func(o *RetryConfig) {
		o.MaxJitter = maxJitter
	}}
}
