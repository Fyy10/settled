package config

import (
	"bytes"
	"encoding/base64"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := load(mapLookup(requiredEnvironment()))
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}

	if cfg.AppEnv != Development {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, Development)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.DatabaseURL != "postgres://settled:test@localhost:5432/settled" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, developmentOrigins) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, developmentOrigins)
	}
	if !bytes.Equal(cfg.JWTSecret, bytes.Repeat([]byte{'j'}, minimumSecretBytes)) {
		t.Error("JWTSecret does not match the decoded environment value")
	}
	if !bytes.Equal(cfg.CSRFSecret, bytes.Repeat([]byte{'c'}, minimumSecretBytes)) {
		t.Error("CSRFSecret does not match the decoded environment value")
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
	if cfg.CookieSameSite != http.SameSiteLaxMode {
		t.Errorf("CookieSameSite = %v, want lax", cfg.CookieSameSite)
	}
	if cfg.CookieDomain != "" {
		t.Errorf("CookieDomain = %q, want empty", cfg.CookieDomain)
	}
	if cfg.DBMaxOpenConns != 10 {
		t.Errorf("DBMaxOpenConns = %d, want 10", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 10 {
		t.Errorf("DBMaxIdleConns = %d, want 10", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 30*time.Minute {
		t.Errorf("DBConnMaxLifetime = %v, want 30m", cfg.DBConnMaxLifetime)
	}
	if cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Errorf("DBConnMaxIdleTime = %v, want 5m", cfg.DBConnMaxIdleTime)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoadProductionDefaults(t *testing.T) {
	t.Parallel()

	values := requiredEnvironment()
	values["APP_ENV"] = "production"
	values["ALLOWED_ORIGINS"] = "https://WEB.Example:443"

	cfg, err := load(mapLookup(values))
	if err != nil {
		t.Fatalf("load production defaults: %v", err)
	}

	if !cfg.CookieSecure {
		t.Error("CookieSecure = false, want true")
	}
	if cfg.CookieSameSite != http.SameSiteNoneMode {
		t.Errorf("CookieSameSite = %v, want none", cfg.CookieSameSite)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{"https://web.example"}) {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
}

func TestLoadTestDefaults(t *testing.T) {
	t.Parallel()

	values := requiredEnvironment()
	values["APP_ENV"] = "test"

	cfg, err := load(mapLookup(values))
	if err != nil {
		t.Fatalf("load test defaults: %v", err)
	}

	if cfg.AllowedOrigins != nil {
		t.Errorf("AllowedOrigins = %v, want nil", cfg.AllowedOrigins)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
	if cfg.CookieSameSite != http.SameSiteLaxMode {
		t.Errorf("CookieSameSite = %v, want lax", cfg.CookieSameSite)
	}
}

func TestLoadCustomValues(t *testing.T) {
	t.Parallel()

	values := requiredEnvironment()
	values["APP_ENV"] = "test"
	values["HTTP_ADDR"] = " 127.0.0.1:9090 "
	values["ALLOWED_ORIGINS"] = " HTTP://Example.COM:080,http://[::1]:08080 "
	values["JWT_SECRET_BASE64"] = encodeSecret('x', 33)
	values["CSRF_SECRET_BASE64"] = encodeSecret('y', 34)
	values["COOKIE_SECURE"] = "true"
	values["COOKIE_SAME_SITE"] = "strict"
	values["COOKIE_DOMAIN"] = " example.test "
	values["DB_MAX_OPEN_CONNS"] = "24"
	values["DB_MAX_IDLE_CONNS"] = "6"
	values["DB_CONN_MAX_LIFETIME"] = "45m"
	values["DB_CONN_MAX_IDLE_TIME"] = "90s"
	values["LOG_LEVEL"] = "debug"

	cfg, err := load(mapLookup(values))
	if err != nil {
		t.Fatalf("load custom values: %v", err)
	}

	if cfg.AppEnv != Test {
		t.Errorf("AppEnv = %q, want test", cfg.AppEnv)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if !reflect.DeepEqual(
		cfg.AllowedOrigins,
		[]string{"http://example.com", "http://[::1]:8080"},
	) {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
	if cfg.CookieSameSite != http.SameSiteStrictMode {
		t.Errorf("CookieSameSite = %v, want strict", cfg.CookieSameSite)
	}
	if cfg.CookieDomain != "example.test" {
		t.Errorf("CookieDomain = %q", cfg.CookieDomain)
	}
	if cfg.DBMaxOpenConns != 24 || cfg.DBMaxIdleConns != 6 {
		t.Errorf(
			"pool sizes = (%d, %d), want (24, 6)",
			cfg.DBMaxOpenConns,
			cfg.DBMaxIdleConns,
		)
	}
	if cfg.DBConnMaxLifetime != 45*time.Minute {
		t.Errorf("DBConnMaxLifetime = %v", cfg.DBConnMaxLifetime)
	}
	if cfg.DBConnMaxIdleTime != 90*time.Second {
		t.Errorf("DBConnMaxIdleTime = %v", cfg.DBConnMaxIdleTime)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

func TestLoadReadsEachEnvironmentVariableOnce(t *testing.T) {
	t.Parallel()

	values := requiredEnvironment()
	counts := make(map[string]int)
	lookup := func(name string) (string, bool) {
		counts[name]++
		value, ok := values[name]
		return value, ok
	}

	if _, err := load(lookup); err != nil {
		t.Fatalf("load: %v", err)
	}

	for _, name := range environmentVariableNames() {
		if counts[name] != 1 {
			t.Errorf("%s read %d times, want once", name, counts[name])
		}
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		change      func(map[string]string)
		wantInError string
	}{
		{
			name: "blank app environment",
			change: func(values map[string]string) {
				values["APP_ENV"] = " "
			},
			wantInError: "APP_ENV",
		},
		{
			name: "unknown app environment",
			change: func(values map[string]string) {
				values["APP_ENV"] = "staging"
			},
			wantInError: "APP_ENV",
		},
		{
			name: "blank HTTP address",
			change: func(values map[string]string) {
				values["HTTP_ADDR"] = " "
			},
			wantInError: "HTTP_ADDR",
		},
		{
			name: "missing database URL",
			change: func(values map[string]string) {
				delete(values, "DATABASE_URL")
			},
			wantInError: "DATABASE_URL",
		},
		{
			name: "blank database URL",
			change: func(values map[string]string) {
				values["DATABASE_URL"] = " "
			},
			wantInError: "DATABASE_URL",
		},
		{
			name: "missing JWT secret",
			change: func(values map[string]string) {
				delete(values, "JWT_SECRET_BASE64")
			},
			wantInError: "JWT_SECRET_BASE64",
		},
		{
			name: "malformed JWT secret",
			change: func(values map[string]string) {
				values["JWT_SECRET_BASE64"] = "not base64"
			},
			wantInError: "JWT_SECRET_BASE64",
		},
		{
			name: "short JWT secret",
			change: func(values map[string]string) {
				values["JWT_SECRET_BASE64"] = encodeSecret('j', minimumSecretBytes-1)
			},
			wantInError: "JWT_SECRET_BASE64",
		},
		{
			name: "missing CSRF secret",
			change: func(values map[string]string) {
				delete(values, "CSRF_SECRET_BASE64")
			},
			wantInError: "CSRF_SECRET_BASE64",
		},
		{
			name: "malformed CSRF secret",
			change: func(values map[string]string) {
				values["CSRF_SECRET_BASE64"] = "not base64"
			},
			wantInError: "CSRF_SECRET_BASE64",
		},
		{
			name: "short CSRF secret",
			change: func(values map[string]string) {
				values["CSRF_SECRET_BASE64"] = encodeSecret('c', minimumSecretBytes-1)
			},
			wantInError: "CSRF_SECRET_BASE64",
		},
		{
			name: "equal secrets",
			change: func(values map[string]string) {
				values["CSRF_SECRET_BASE64"] = values["JWT_SECRET_BASE64"]
			},
			wantInError: "different",
		},
		{
			name: "blank origin entry",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://one.example,,https://two.example"
			},
			wantInError: "blank",
		},
		{
			name: "origin path",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://web.example/"
			},
			wantInError: "origin",
		},
		{
			name: "origin query",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://web.example?source=test"
			},
			wantInError: "origin",
		},
		{
			name: "origin fragment",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://web.example#fragment"
			},
			wantInError: "origin",
		},
		{
			name: "origin user information",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://user@web.example"
			},
			wantInError: "origin",
		},
		{
			name: "origin wildcard",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://*.example"
			},
			wantInError: "wildcard",
		},
		{
			name: "origin scheme",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "ftp://web.example"
			},
			wantInError: "scheme",
		},
		{
			name: "origin port",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://web.example:99999"
			},
			wantInError: "port",
		},
		{
			name: "empty origin port",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://web.example:"
			},
			wantInError: "port",
		},
		{
			name: "duplicate canonical origin",
			change: func(values map[string]string) {
				values["ALLOWED_ORIGINS"] = "https://WEB.example:443,https://web.example"
			},
			wantInError: "duplicate",
		},
		{
			name: "production missing origins",
			change: func(values map[string]string) {
				values["APP_ENV"] = "production"
			},
			wantInError: "ALLOWED_ORIGINS",
		},
		{
			name: "production HTTP origin",
			change: func(values map[string]string) {
				values["APP_ENV"] = "production"
				values["ALLOWED_ORIGINS"] = "http://web.example"
			},
			wantInError: "HTTPS",
		},
		{
			name: "invalid cookie secure",
			change: func(values map[string]string) {
				values["COOKIE_SECURE"] = "yes"
			},
			wantInError: "COOKIE_SECURE",
		},
		{
			name: "invalid cookie same site",
			change: func(values map[string]string) {
				values["COOKIE_SAME_SITE"] = "default"
			},
			wantInError: "COOKIE_SAME_SITE",
		},
		{
			name: "same site none without secure",
			change: func(values map[string]string) {
				values["COOKIE_SAME_SITE"] = "none"
			},
			wantInError: "COOKIE_SECURE",
		},
		{
			name: "production insecure cookie",
			change: func(values map[string]string) {
				values["APP_ENV"] = "production"
				values["ALLOWED_ORIGINS"] = "https://web.example"
				values["COOKIE_SECURE"] = "false"
			},
			wantInError: "COOKIE_SECURE",
		},
		{
			name: "production lax cookie",
			change: func(values map[string]string) {
				values["APP_ENV"] = "production"
				values["ALLOWED_ORIGINS"] = "https://web.example"
				values["COOKIE_SAME_SITE"] = "lax"
			},
			wantInError: "COOKIE_SAME_SITE",
		},
		{
			name: "noninteger max open",
			change: func(values map[string]string) {
				values["DB_MAX_OPEN_CONNS"] = "many"
			},
			wantInError: "DB_MAX_OPEN_CONNS",
		},
		{
			name: "zero max open",
			change: func(values map[string]string) {
				values["DB_MAX_OPEN_CONNS"] = "0"
			},
			wantInError: "DB_MAX_OPEN_CONNS",
		},
		{
			name: "negative max idle",
			change: func(values map[string]string) {
				values["DB_MAX_IDLE_CONNS"] = "-1"
			},
			wantInError: "DB_MAX_IDLE_CONNS",
		},
		{
			name: "max idle above max open",
			change: func(values map[string]string) {
				values["DB_MAX_OPEN_CONNS"] = "4"
				values["DB_MAX_IDLE_CONNS"] = "5"
			},
			wantInError: "DB_MAX_IDLE_CONNS",
		},
		{
			name: "invalid max lifetime",
			change: func(values map[string]string) {
				values["DB_CONN_MAX_LIFETIME"] = "later"
			},
			wantInError: "DB_CONN_MAX_LIFETIME",
		},
		{
			name: "nonpositive max lifetime",
			change: func(values map[string]string) {
				values["DB_CONN_MAX_LIFETIME"] = "0s"
			},
			wantInError: "DB_CONN_MAX_LIFETIME",
		},
		{
			name: "invalid max idle time",
			change: func(values map[string]string) {
				values["DB_CONN_MAX_IDLE_TIME"] = "-1s"
			},
			wantInError: "DB_CONN_MAX_IDLE_TIME",
		},
		{
			name: "invalid log level",
			change: func(values map[string]string) {
				values["LOG_LEVEL"] = "trace"
			},
			wantInError: "LOG_LEVEL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			values := requiredEnvironment()
			test.change(values)
			_, err := load(mapLookup(values))
			if err == nil {
				t.Fatal("load succeeded, want error")
			}
			if !strings.Contains(err.Error(), test.wantInError) {
				t.Errorf("error = %q, want it to contain %q", err, test.wantInError)
			}
		})
	}
}

func TestLoadErrorsDoNotExposeConfigurationValues(t *testing.T) {
	t.Parallel()

	values := requiredEnvironment()
	values["DATABASE_URL"] = "postgres://private-user:private-password@secret-host/database"
	values["JWT_SECRET_BASE64"] = "private-invalid-secret"

	_, err := load(mapLookup(values))
	if err == nil {
		t.Fatal("load succeeded, want error")
	}
	for _, privateValue := range []string{
		values["DATABASE_URL"],
		values["JWT_SECRET_BASE64"],
		"private-password",
	} {
		if strings.Contains(err.Error(), privateValue) {
			t.Errorf("error contains private value %q: %v", privateValue, err)
		}
	}
}

func requiredEnvironment() map[string]string {
	return map[string]string{
		"DATABASE_URL":       "postgres://settled:test@localhost:5432/settled",
		"JWT_SECRET_BASE64":  encodeSecret('j', minimumSecretBytes),
		"CSRF_SECRET_BASE64": encodeSecret('c', minimumSecretBytes),
	}
}

func encodeSecret(value byte, size int) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value}, size))
}

func mapLookup(values map[string]string) lookupEnv {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func environmentVariableNames() []string {
	return []string{
		"APP_ENV",
		"HTTP_ADDR",
		"DATABASE_URL",
		"ALLOWED_ORIGINS",
		"JWT_SECRET_BASE64",
		"CSRF_SECRET_BASE64",
		"COOKIE_SECURE",
		"COOKIE_SAME_SITE",
		"COOKIE_DOMAIN",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME",
		"DB_CONN_MAX_IDLE_TIME",
		"LOG_LEVEL",
	}
}
