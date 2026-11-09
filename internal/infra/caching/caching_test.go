package caching

import "testing"

func TestKeyHelpers(t *testing.T) {
	tests := map[string]string{
		AuthRequestKey("req"):            "authreq:req",
		AuthCodeKey("abc"):               "authcode:abc",
		MFAKey("tok"):                    "mfa:tok",
		SessionKey("hash"):               "sess:hash",
		JWKSKey("default"):               "jwks:default",
		RateLimitKey("login", "1.2.3.4"): "rl:login:1.2.3.4",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("key = %q, want %q", got, want)
		}
	}
}
