package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// HTTPLogoutSender posts a signed logout token to a registered HTTPS URI. The
// dialer resolves the host and connects only to a public address, which blocks
// loopback, private, link-local, and DNS-rebinding targets. Redirects are not
// followed.
type HTTPLogoutSender struct {
	client *http.Client
}

func NewLogoutSender() *HTTPLogoutSender {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialPublicOnly
	return &HTTPLogoutSender{
		client: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *HTTPLogoutSender) Send(ctx context.Context, uri, token string) error {
	if err := validateLogoutURI(ctx, uri); err != nil {
		return err
	}
	form := url.Values{"logout_token": {token}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uri,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("logout endpoint returned %d", response.StatusCode)
	}
	return nil
}

func validateLogoutURI(ctx context.Context, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse logout uri: %w", err)
	}
	if parsed.Scheme != "https" {
		return errors.New("logout uri must use https")
	}
	if parsed.Host == "" || parsed.User != nil {
		return errors.New("logout uri has an invalid host")
	}
	_, err = resolvePublic(ctx, parsed.Hostname())
	return err
}

func dialPublicOnly(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ip, err := resolvePublic(ctx, host)
	if err != nil {
		return nil, err
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

func resolvePublic(ctx context.Context, host string) (netip.Addr, error) {
	if host == "" {
		return netip.Addr{}, errors.New("host is empty")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("resolve logout host: %w", err)
	}
	if len(addresses) == 0 {
		return netip.Addr{}, errors.New("logout host has no address")
	}
	for _, address := range addresses {
		if !isPublicIP(address) {
			return netip.Addr{}, errors.New("logout host resolves to a non-public address")
		}
	}
	return addresses[0], nil
}

func isPublicIP(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() ||
		address.IsPrivate() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsMulticast() {
		return false
	}
	// Carrier-grade NAT is not covered by IsPrivate.
	if address.Is4() {
		cgnat := netip.MustParsePrefix("100.64.0.0/10")
		if cgnat.Contains(address) {
			return false
		}
	}
	return true
}
