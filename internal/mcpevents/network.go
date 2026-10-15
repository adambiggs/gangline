package mcpevents

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

type blockedAddressError struct{}

func (*blockedAddressError) Error() string { return "callback address is not public" }

var blockedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("3fff::/20"),
}

func publicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || (addr.Is6() && !netip.MustParsePrefix("2000::/3").Contains(addr)) {
		return false
	}
	for _, p := range blockedNetworks {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

type addressResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type contextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// checkedDial resolves once per new connection, validates the complete answer,
// and dials only validated literals. HTTP retains the original host for TLS.
func checkedDial(resolver addressResolver, dialer contextDialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		var addresses []netip.Addr
		if ip, err := netip.ParseAddr(host); err == nil {
			addresses = []netip.Addr{ip}
		} else {
			addresses, err = resolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("callback DNS returned no addresses")
		}
		for _, ip := range addresses {
			if !publicAddress(ip) {
				return nil, &blockedAddressError{}
			}
		}
		var last error
		for _, ip := range addresses {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
}
func callbackClient() *http.Client {
	transport := &http.Transport{DialContext: checkedDial(net.DefaultResolver, &net.Dialer{Timeout: 10 * time.Second}), ForceAttemptHTTP2: true, TLSHandshakeTimeout: 10 * time.Second}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
