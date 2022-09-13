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

// Option is the only struct that can be used to set Retry Config.
type Option struct {
	F func(o *Config)
}

// WithMaxAttemptTimes set WithMaxAttemptTimes , including the first call.
func WithMaxAttemptTimes(maxAttemptTimes uint) Option {
	return Option{F: func(o *Config) {
		o.MaxAttemptTimes = maxAttemptTimes
	}}
}

// WithInitDelay set init Delay.
func WithInitDelay(delay time.Duration) Option {
	return Option{F: func(o *Config) {
		o.Delay = delay
	}}
}

// WithMaxDelay set MaxDelay.
func WithMaxDelay(maxDelay time.Duration) Option {
	return Option{F: func(o *Config) {
		o.MaxDelay = maxDelay
	}}
}

// WithDelayPolicy set DelayPolicy.
func WithDelayPolicy(delayPolicy DelayPolicyFunc) Option {
	return Option{F: func(o *Config) {
		o.DelayPolicy = delayPolicy
	}}
}

// WithMaxJitter set MaxJitter.
func WithMaxJitter(maxJitter time.Duration) Option {
	return Option{F: func(o *Config) {
		o.MaxJitter = maxJitter
	}}
}
