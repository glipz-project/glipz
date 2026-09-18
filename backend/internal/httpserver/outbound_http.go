package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

func newPublicOutboundHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = publicOutboundDialContext
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := validatePublicOutboundURL(req.Context(), req.URL.String(), true)
			return err
		},
	}
}

func validatePublicOutboundURL(ctx context.Context, raw string, allowHTTP bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return nil, fmt.Errorf("invalid url")
	}
	if u.User != nil || strings.TrimSpace(u.Host) == "" {
		return nil, fmt.Errorf("invalid host")
	}
	switch strings.ToLower(strings.TrimSpace(u.Scheme)) {
	case "https":
	case "http":
		if !allowHTTP {
			return nil, fmt.Errorf("http not allowed")
		}
	default:
		return nil, fmt.Errorf("unsupported scheme")
	}
	if err := ensurePublicOutboundHost(ctx, u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

func ensurePublicOutboundHost(ctx context.Context, host string) error {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return fmt.Errorf("empty host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".local") {
		return fmt.Errorf("local host not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicOutboundIP(ip) {
			return fmt.Errorf("private ip not allowed")
		}
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("dns lookup failed")
	}
	for _, addr := range addrs {
		if !isPublicOutboundIP(addr.IP) {
			return fmt.Errorf("private ip not allowed")
		}
	}
	return nil
}

func isPublicOutboundIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "::/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/32", "2001:db8::/32", "2002::/16"} {
		if netip.MustParsePrefix(raw).Contains(addr) {
			return false
		}
	}
	return !(ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified())
}

func isSafeRemoteHost(host string) bool {
	h := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(strings.ToLower(host)), "https://"), "http://")
	h = strings.Trim(strings.TrimSuffix(h, "/"), "[]")
	if h == "" || h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") {
		return false
	}
	return net.ParseIP(h) == nil
}

func publicOutboundDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if err := ensurePublicOutboundHost(ctx, host); err != nil {
		return nil, err
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("dns lookup failed")
	}
	dialer := &net.Dialer{}
	var lastErr error
	for _, addr := range addrs {
		if !isPublicOutboundIP(addr.IP) {
			return nil, fmt.Errorf("private ip not allowed")
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("dial failed")
}
