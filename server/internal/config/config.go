package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Environment string

const (
	Development Environment = "development"
	Test        Environment = "test"
	Production  Environment = "production"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultDBMaxOpenConns    = 10
	defaultDBMaxIdleConns    = 10
	defaultDBConnMaxLifetime = 30 * time.Minute
	defaultDBConnMaxIdleTime = 5 * time.Minute
	minimumSecretBytes       = 32
)

var developmentOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

type Config struct {
	AppEnv            Environment
	HTTPAddr          string
	DatabaseURL       string
	AllowedOrigins    []string
	JWTSecret         []byte
	CSRFSecret        []byte
	CookieSecure      bool
	CookieSameSite    http.SameSite
	CookieDomain      string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
	LogLevel          slog.Level
}

type lookupEnv func(string) (string, bool)

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup lookupEnv) (Config, error) {
	appEnv, err := loadEnvironment(lookup)
	if err != nil {
		return Config{}, err
	}

	httpAddr, err := loadString(lookup, "HTTP_ADDR", defaultHTTPAddr)
	if err != nil {
		return Config{}, err
	}
	if httpAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be blank")
	}

	databaseURL, err := loadRequiredString(lookup, "DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	allowedOrigins, err := loadAllowedOrigins(lookup, appEnv)
	if err != nil {
		return Config{}, err
	}

	jwtSecret, err := loadSecret(lookup, "JWT_SECRET_BASE64")
	if err != nil {
		return Config{}, err
	}
	csrfSecret, err := loadSecret(lookup, "CSRF_SECRET_BASE64")
	if err != nil {
		return Config{}, err
	}
	if bytes.Equal(jwtSecret, csrfSecret) {
		return Config{}, fmt.Errorf("JWT_SECRET_BASE64 and CSRF_SECRET_BASE64 must decode to different values")
	}

	cookieSecureDefault := appEnv == Production
	cookieSecure, err := loadBool(lookup, "COOKIE_SECURE", cookieSecureDefault)
	if err != nil {
		return Config{}, err
	}

	cookieSameSiteDefault := "lax"
	if appEnv == Production {
		cookieSameSiteDefault = "none"
	}
	cookieSameSite, err := loadSameSite(lookup, cookieSameSiteDefault)
	if err != nil {
		return Config{}, err
	}
	if cookieSameSite == http.SameSiteNoneMode && !cookieSecure {
		return Config{}, fmt.Errorf("COOKIE_SECURE must be true when COOKIE_SAME_SITE is none")
	}

	cookieDomain, err := loadString(lookup, "COOKIE_DOMAIN", "")
	if err != nil {
		return Config{}, err
	}

	maxOpenConns, err := loadInt(lookup, "DB_MAX_OPEN_CONNS", defaultDBMaxOpenConns)
	if err != nil {
		return Config{}, err
	}
	if maxOpenConns <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_OPEN_CONNS must be a positive integer")
	}

	maxIdleConns, err := loadInt(lookup, "DB_MAX_IDLE_CONNS", defaultDBMaxIdleConns)
	if err != nil {
		return Config{}, err
	}
	if maxIdleConns < 0 || maxIdleConns > maxOpenConns {
		return Config{}, fmt.Errorf("DB_MAX_IDLE_CONNS must be between zero and DB_MAX_OPEN_CONNS")
	}

	connMaxLifetime, err := loadDuration(
		lookup,
		"DB_CONN_MAX_LIFETIME",
		defaultDBConnMaxLifetime,
	)
	if err != nil {
		return Config{}, err
	}
	connMaxIdleTime, err := loadDuration(
		lookup,
		"DB_CONN_MAX_IDLE_TIME",
		defaultDBConnMaxIdleTime,
	)
	if err != nil {
		return Config{}, err
	}

	logLevel, err := loadLogLevel(lookup)
	if err != nil {
		return Config{}, err
	}

	if appEnv == Production {
		if len(allowedOrigins) == 0 {
			return Config{}, fmt.Errorf("ALLOWED_ORIGINS is required in production")
		}
		if !cookieSecure {
			return Config{}, fmt.Errorf("COOKIE_SECURE must be true in production")
		}
		if cookieSameSite != http.SameSiteNoneMode {
			return Config{}, fmt.Errorf("COOKIE_SAME_SITE must be none in production")
		}
	}

	return Config{
		AppEnv:            appEnv,
		HTTPAddr:          httpAddr,
		DatabaseURL:       databaseURL,
		AllowedOrigins:    allowedOrigins,
		JWTSecret:         jwtSecret,
		CSRFSecret:        csrfSecret,
		CookieSecure:      cookieSecure,
		CookieSameSite:    cookieSameSite,
		CookieDomain:      cookieDomain,
		DBMaxOpenConns:    maxOpenConns,
		DBMaxIdleConns:    maxIdleConns,
		DBConnMaxLifetime: connMaxLifetime,
		DBConnMaxIdleTime: connMaxIdleTime,
		LogLevel:          logLevel,
	}, nil
}

func loadEnvironment(lookup lookupEnv) (Environment, error) {
	value, ok := lookup("APP_ENV")
	if !ok {
		return Development, nil
	}

	switch Environment(strings.TrimSpace(value)) {
	case Development:
		return Development, nil
	case Test:
		return Test, nil
	case Production:
		return Production, nil
	default:
		return "", fmt.Errorf("APP_ENV must be development, test, or production")
	}
}

func loadString(lookup lookupEnv, name, defaultValue string) (string, error) {
	value, ok := lookup(name)
	if !ok {
		return defaultValue, nil
	}
	return strings.TrimSpace(value), nil
}

func loadRequiredString(lookup lookupEnv, name string) (string, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return strings.TrimSpace(value), nil
}

func loadSecret(lookup lookupEnv, name string) ([]byte, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("%s is required", name)
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("%s must be valid standard Base64", name)
	}
	if len(decoded) < minimumSecretBytes {
		return nil, fmt.Errorf("%s must decode to at least %d bytes", name, minimumSecretBytes)
	}
	return decoded, nil
}

func loadAllowedOrigins(lookup lookupEnv, appEnv Environment) ([]string, error) {
	value, ok := lookup("ALLOWED_ORIGINS")
	if !ok {
		if appEnv == Development {
			return append([]string(nil), developmentOrigins...), nil
		}
		return nil, nil
	}
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("ALLOWED_ORIGINS must not contain blank entries")
		}
		origin, err := canonicalOrigin(strings.TrimSpace(part), appEnv == Production)
		if err != nil {
			return nil, fmt.Errorf("ALLOWED_ORIGINS contains an invalid origin: %w", err)
		}
		if _, duplicate := seen[origin]; duplicate {
			return nil, fmt.Errorf("ALLOWED_ORIGINS must not contain duplicate origins")
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins, nil
}

func canonicalOrigin(value string, requireHTTPS bool) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("origin must be an absolute HTTP or HTTPS URL")
	}
	if parsed.Opaque != "" ||
		parsed.User != nil ||
		parsed.Host == "" ||
		parsed.Path != "" ||
		parsed.RawPath != "" ||
		parsed.RawQuery != "" ||
		parsed.ForceQuery ||
		parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme, host, and optional port")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin scheme must be HTTP or HTTPS")
	}
	if requireHTTPS && scheme != "https" {
		return "", fmt.Errorf("production origins must use HTTPS")
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return "", fmt.Errorf("origin must include a hostname")
	}
	if strings.Contains(hostname, "*") {
		return "", fmt.Errorf("origin hostname must not contain a wildcard")
	}
	if strings.Contains(value, "#") {
		return "", fmt.Errorf("origin must not contain a fragment")
	}

	port := parsed.Port()
	if strings.HasSuffix(parsed.Host, ":") {
		return "", fmt.Errorf("origin port must not be empty")
	}
	if port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return "", fmt.Errorf("origin port must be between 1 and 65535")
		}
		if (scheme == "http" && portNumber == 80) ||
			(scheme == "https" && portNumber == 443) {
			port = ""
		} else {
			port = strconv.Itoa(portNumber)
		}
	}

	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	return scheme + "://" + host, nil
}

func loadBool(lookup lookupEnv, name string, defaultValue bool) (bool, error) {
	value, ok := lookup(name)
	if !ok {
		return defaultValue, nil
	}

	switch strings.TrimSpace(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", name)
	}
}

func loadSameSite(lookup lookupEnv, defaultValue string) (http.SameSite, error) {
	value, ok := lookup("COOKIE_SAME_SITE")
	if !ok {
		value = defaultValue
	}

	switch strings.TrimSpace(value) {
	case "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return http.SameSiteDefaultMode, fmt.Errorf(
			"COOKIE_SAME_SITE must be lax, strict, or none",
		)
	}
}

func loadInt(lookup lookupEnv, name string, defaultValue int) (int, error) {
	value, ok := lookup(name)
	if !ok {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func loadDuration(
	lookup lookupEnv,
	name string,
	defaultValue time.Duration,
) (time.Duration, error) {
	value, ok := lookup(name)
	if !ok {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func loadLogLevel(lookup lookupEnv) (slog.Level, error) {
	value, ok := lookup("LOG_LEVEL")
	if !ok {
		return slog.LevelInfo, nil
	}

	switch strings.TrimSpace(value) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
}
