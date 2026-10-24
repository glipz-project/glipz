package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const identityTransferHTTPTimeout = 20 * time.Second

func normalizeTransferOrigin(raw string) (string, bool, error) {
	return normalizeTransferOriginWithLoopback(raw, false)
}

func (s *Server) normalizeTransferOrigin(raw string) (string, bool, error) {
	return normalizeTransferOriginWithLoopback(raw, s.cfg.IdentityTransferAllowLoopback)
}

func normalizeTransferOriginWithLoopback(raw string, allowLoopback bool) (string, bool, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false, err
	}
	if u.Scheme == "" || u.Host == "" || u.User != nil {
		return "", false, fmt.Errorf("invalid origin")
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", false, fmt.Errorf("origin must not include path, query, or fragment")
	}
	host := u.Hostname()
	isLocal := isLoopbackHost(host)
	if isLocal && !allowLoopback {
		return "", false, fmt.Errorf("loopback origin is disabled")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !isLocal {
			return "", false, fmt.Errorf("http origin is only allowed for local development")
		}
	default:
		return "", false, fmt.Errorf("unsupported origin scheme")
	}
	if !isLocal {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rejectUnsafeResolvedHost(ctx, host); err != nil {
			return "", false, err
		}
	}
	u.Path, u.RawQuery, u.Fragment = "", "", ""
	return strings.TrimRight(u.String(), "/"), isLocal, nil
}

func transferURL(origin, path string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(origin), "/"))
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid origin")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base.Path = path
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func (s *Server) newIdentityTransferHTTPClient() *http.Client {
	c := newPublicOutboundHTTPClient(identityTransferHTTPTimeout)
	if s.cfg.IdentityTransferAllowLoopback {
		c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if isLoopbackHost(host) {
				// Pin localhost to a literal loopback address rather than resolving DNS.
				if strings.EqualFold(host, "localhost") {
					host = "127.0.0.1"
				}
				return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(host, port))
			}
			return publicOutboundDialContext(ctx, network, address)
		}
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

func (s *Server) validateTransferRequestURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if u.User != nil || u.Fragment != "" {
		return fmt.Errorf("invalid transfer URL")
	}
	origin := &url.URL{Scheme: u.Scheme, Host: u.Host}
	_, _, err = s.normalizeTransferOrigin(origin.String())
	return err
}

func rejectUnsafeResolvedHost(ctx context.Context, host string) error {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("host has no addresses")
	}
	for _, addr := range ips {
		if isUnsafeRemoteIP(addr.IP) {
			return fmt.Errorf("origin resolves to a private or reserved address")
		}
	}
	return nil
}

func isLoopbackHost(host string) bool {
	h := strings.Trim(strings.ToLower(host), "[]")
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func isUnsafeRemoteIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return !isPublicOutboundIP(ip)
}
