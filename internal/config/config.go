package config

const DefaultUserAgent = "ModelContextProtocol/1.0 (Autonomous; +https://github.com/modelcontextprotocol/servers)"

// BrowserUserAgent can be passed via --user-agent for user-initiated browsing mode.
const BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type Config struct {
	UserAgent       string
	ProxyURL        string
	IgnoreRobotsTxt bool
}

func DefaultConfig() *Config {
	return &Config{
		UserAgent: DefaultUserAgent,
	}
}
