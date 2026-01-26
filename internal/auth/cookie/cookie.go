package cookie

type Seconds = int

type CookieConfig struct {
	SessionLifetime Seconds
	Secure          bool
	Path            string
	HTTPOnly        bool
	Domain          string
}

func DefaultCookieConfig() *CookieConfig {
	return &CookieConfig{
		SessionLifetime: 604800, // 7 weeks
		Secure:          true,
		Path:            "/",
		HTTPOnly:        true,
		Domain:          "",
	}
}
