package retry

import "time"

type RetryOption struct {

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
