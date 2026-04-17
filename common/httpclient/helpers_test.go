package httpclient

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRequestAuthority(t *testing.T) {
	testCases := []struct {
		name   string
		url    string
		expect string
	}{
		{name: "https default port", url: "https://example.com/foo", expect: "example.com:443"},
		{name: "http default port", url: "http://example.com/foo", expect: "example.com:80"},
		{name: "https explicit port", url: "https://example.com:8443/foo", expect: "example.com:8443"},
		{name: "https uppercase host", url: "https://EXAMPLE.COM/foo", expect: "example.com:443"},
		{name: "https ipv6 default port", url: "https://[2001:db8::1]/foo", expect: "[2001:db8::1]:443"},
		{name: "https ipv6 explicit port", url: "https://[2001:db8::1]:8443/foo", expect: "[2001:db8::1]:8443"},
		{name: "https ipv4", url: "https://192.0.2.1/foo", expect: "192.0.2.1:443"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, err := url.Parse(testCase.url)
			if err != nil {
				t.Fatalf("parse url: %v", err)
			}
			got := requestAuthority(&http.Request{URL: parsed})
			if got != testCase.expect {
				t.Fatalf("got %q, want %q", got, testCase.expect)
			}
		})
	}

	t.Run("nil request", func(t *testing.T) {
		if got := requestAuthority(nil); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
	t.Run("nil URL", func(t *testing.T) {
		if got := requestAuthority(&http.Request{}); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
	t.Run("empty host", func(t *testing.T) {
		if got := requestAuthority(&http.Request{URL: &url.URL{Scheme: "https"}}); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}
