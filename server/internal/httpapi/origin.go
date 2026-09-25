package httpapi

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type OriginPolicy struct {
	allowed map[string]struct{}
}

func NewOriginPolicy(origins []string) (OriginPolicy, error) {
	allowed := make(map[string]struct{}, len(origins))
	for _, value := range origins {
		origin, err := canonicalOrigin(value)
		if err != nil {
			return OriginPolicy{}, fmt.Errorf("invalid allowed origin: %w", err)
		}
		if _, duplicate := allowed[origin]; duplicate {
			return OriginPolicy{}, fmt.Errorf("duplicate allowed origin %q", origin)
		}
		allowed[origin] = struct{}{}
	}
	return OriginPolicy{allowed: allowed}, nil
}

func (policy OriginPolicy) allows(value string) (string, bool) {
	origin, err := canonicalOrigin(value)
	if err != nil {
		return "", false
	}
	_, allowed := policy.allowed[origin]
	return origin, allowed
}

func canonicalOrigin(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("origin must not be blank or padded")
	}

	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.Opaque != "" ||
		parsed.User != nil ||
		parsed.Host == "" ||
		parsed.Path != "" ||
		parsed.RawPath != "" ||
		parsed.RawQuery != "" ||
		parsed.ForceQuery ||
		parsed.Fragment != "" ||
		strings.Contains(value, "#") {
		return "", fmt.Errorf("origin must contain only scheme, host, and optional port")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin scheme must be HTTP or HTTPS")
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" || strings.Contains(hostname, "*") {
		return "", fmt.Errorf("origin must contain a concrete hostname")
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
