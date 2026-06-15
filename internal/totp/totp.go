package totp

// Config 是一组 TOTP 参数。零值不可用——用 DefaultConfig 拿对齐 Google
// Authenticator 的默认值再覆盖。
type Config struct {
	Digits    int       // 输出位数（6 或 8）
	Period    int       // 时间步长，秒（默认 30）
	Algorithm Algorithm // HMAC 算法（默认 SHA1）
}

// DefaultConfig 返回对齐 Google Authenticator / Authy / 1Password 的默认参数：
// SHA1 / 6 位 / 30 秒。绝大多数服务直接能用。
func DefaultConfig() Config {
	return Config{Digits: 6, Period: 30, Algorithm: AlgoSHA1}
}

// normalize 把零值字段补成默认值，让 Config 部分填充也能用。
func (c Config) normalize() Config {
	if c.Digits == 0 {
		c.Digits = 6
	}
	if c.Period == 0 {
		c.Period = 30
	}
	if c.Algorithm == "" {
		c.Algorithm = AlgoSHA1
	}
	return c
}

// counter 把 Unix 时间戳换算成 HOTP 计数器（time / period）。
func (c Config) counter(unixTime int64) uint64 {
	return uint64(unixTime / int64(c.Period))
}

// GenerateAt 计算给定 Unix 时间戳的 TOTP 码。
func GenerateAt(key []byte, unixTime int64, cfg Config) string {
	cfg = cfg.normalize()
	return HOTP(key, cfg.counter(unixTime), cfg.Digits, cfg.Algorithm)
}

// ExpiresInAt 返回当前码还剩多少秒到期（基于 unixTime）。
func ExpiresInAt(unixTime int64, cfg Config) int {
	cfg = cfg.normalize()
	return cfg.Period - int(unixTime%int64(cfg.Period))
}

// VerifyAt 验证一个码在 unixTime 附近是否有效。
// window 表示向前 / 向后各容许多少个时间窗（对齐时钟漂移）；window=1 检查
// 当前窗 + 前后各 1 窗，共 3 个窗。用 hmac 风格的常量比较避免时序泄露。
func VerifyAt(key []byte, code string, unixTime int64, window int, cfg Config) bool {
	cfg = cfg.normalize()
	if window < 0 {
		window = 0
	}
	base := cfg.counter(unixTime)
	for delta := -window; delta <= window; delta++ {
		c := int64(base) + int64(delta)
		if c < 0 {
			continue
		}
		candidate := HOTP(key, uint64(c), cfg.Digits, cfg.Algorithm)
		if constantTimeEqual(candidate, code) {
			return true
		}
	}
	return false
}

// constantTimeEqual 常量时间比较两个等长（或不等长）字符串，避免按字节短路
// 泄露匹配进度。
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
