package httpapi

import "testing"

func TestCanonicalOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
	}{
		{value: "https://web.example", want: "https://web.example"},
		{value: "HTTPS://WEB.EXAMPLE", want: "https://web.example"},
		{value: "http://Web.Example:80", want: "http://web.example"},
		{value: "https://Web.Example:443", want: "https://web.example"},
		{value: "https://Web.Example:0444", want: "https://web.example:444"},
		{value: "http://127.0.0.1:5173", want: "http://127.0.0.1:5173"},
		{value: "http://[::1]:80", want: "http://[::1]"},
		{value: "https://[2001:db8::1]:8443", want: "https://[2001:db8::1]:8443"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalOrigin(test.value)
			if err != nil {
				t.Fatalf("canonicalOrigin: %v", err)
			}
			if got != test.want {
				t.Errorf("canonicalOrigin(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestCanonicalOriginRejectsNonOrigins(t *testing.T) {
	t.Parallel()

	values := []string{
		"",
		" ",
		" https://web.example",
		"https://web.example ",
		"web.example",
		"//web.example",
		"ftp://web.example",
		"https://",
		"https:///path",
		"https://user@web.example",
		"https://*.example",
		"https://web.example/",
		"https://web.example/path",
		"https://web.example?",
		"https://web.example?query=value",
		"https://web.example#",
		"https://web.example#fragment",
		"https://web.example:",
		"https://web.example:0",
		"https://web.example:65536",
		"https://web.example:not-a-port",
		"https://first.example https://second.example",
	}

	for _, value := range values {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if got, err := canonicalOrigin(value); err == nil {
				t.Errorf("canonicalOrigin(%q) = %q, want error", value, got)
			}
		})
	}
}

func TestOriginPolicyCanonicalAllowlist(t *testing.T) {
	t.Parallel()

	policy, err := NewOriginPolicy([]string{
		"https://WEB.example:443",
		"http://localhost:5173",
	})
	if err != nil {
		t.Fatalf("NewOriginPolicy: %v", err)
	}

	tests := []struct {
		value         string
		wantCanonical string
		wantAllowed   bool
	}{
		{
			value:         "HTTPS://web.EXAMPLE",
			wantCanonical: "https://web.example",
			wantAllowed:   true,
		},
		{
			value:         "http://LOCALHOST:5173",
			wantCanonical: "http://localhost:5173",
			wantAllowed:   true,
		},
		{value: "https://other.example", wantCanonical: "https://other.example"},
		{value: "not-an-origin"},
	}
	for _, test := range tests {
		canonical, allowed := policy.allows(test.value)
		if allowed != test.wantAllowed || canonical != test.wantCanonical {
			t.Errorf(
				"allows(%q) = (%q, %v), want (%q, %v)",
				test.value,
				canonical,
				allowed,
				test.wantCanonical,
				test.wantAllowed,
			)
		}
	}
}

func TestNewOriginPolicyRejectsInvalidAndCanonicalDuplicates(t *testing.T) {
	t.Parallel()

	if _, err := NewOriginPolicy([]string{"https://web.example/path"}); err == nil {
		t.Error("invalid configured origin was accepted")
	}
	if _, err := NewOriginPolicy([]string{
		"https://WEB.example:443",
		"https://web.example",
	}); err == nil {
		t.Error("canonical duplicate origins were accepted")
	}
}
