package mcpevents

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"testing"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}
func TestPublicAddressPolicy(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "172.16.0.1", "192.168.1.1", "192.0.0.8", "192.0.2.1", "192.88.99.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "255.255.255.255", "0.0.0.0", "::1", "::", "fe80::1", "fc00::1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "2001::1", "2001:db8::1", "2002:a00:1::", "3fff::1"} {
		if publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("nonpublic address accepted: %s", address)
		}
	}
	for _, address := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "2001:4860:4860::8888"} {
		if !publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("public address rejected: %s", address)
		}
	}
}
func TestDNSPinningAndMixedAnswers(t *testing.T) {
	queries := 0
	calls := 0
	resolver := resolverFunc(func(_ context.Context, network, host string) ([]netip.Addr, error) {
		queries++
		if network != "ip" || host != "callback.example" {
			t.Fatal("wrong lookup")
		}
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	})
	sentinel := errors.New("test dial")
	dialer := dialerFunc(func(_ context.Context, network, address string) (net.Conn, error) {
		calls++
		if address != "1.1.1.1:443" || network != "tcp" {
			t.Fatalf("DNS answer was not pinned: %s", address)
		}
		return nil, sentinel
	})
	if _, err := checkedDial(resolver, dialer)(context.Background(), "tcp", "callback.example:443"); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if queries != 1 || calls != 1 {
		t.Fatal("unexpected DNS or dial count")
	}
	// A later connection resolves again, and cannot reuse a stale public answer.
	resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, nil
	})
	_, err := checkedDial(resolver, dialer)(context.Background(), "tcp", "callback.example:443")
	var blocked *blockedAddressError
	if !errors.As(err, &blocked) || calls != 1 {
		t.Fatal("mixed public/private answer reached dial")
	}
	_, err = checkedDial(resolver, dialer)(context.Background(), "tcp", "127.0.0.1:443")
	if !errors.As(err, &blocked) || calls != 1 {
		t.Fatal("private literal reached dial")
	}
	if callbackClient().Transport.(*http.Transport).Proxy != nil {
		t.Fatal("proxy bypasses address checks")
	}
}
