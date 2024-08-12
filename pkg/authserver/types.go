package authserver

type SentryAuthInfo struct {
	Dsn         string `json:"dsn"`
	Environment string `json:"environment"`
}
