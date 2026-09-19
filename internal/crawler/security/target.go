package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrUnsafeHost        = errors.New("unsafe host")
	ErrUnsafeIP          = errors.New("unsafe IP address")
	ErrUnsafePort        = errors.New("unsafe port")
	ErrUserInfo          = errors.New("userinfo is not allowed")
)

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type NetResolver struct{}

func (NetResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, network, host)
}

type Validator struct {
	Resolver     Resolver
	AllowedPorts map[uint16]struct{}
}

type ResolvedTarget struct {
	URL  *url.URL
	Host string
	Port uint16
	IPs  []netip.Addr
}

func NewValidator() Validator {
	return Validator{
		Resolver: NetResolver{},
		AllowedPorts: map[uint16]struct{}{
			80:  {},
			443: {},
		},
	}
}

func (v Validator) Validate(ctx context.Context, raw string) (ResolvedTarget, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ResolvedTarget{}, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ResolvedTarget{}, ErrUnsupportedScheme
	}
	if u.User != nil {
		return ResolvedTarget{}, ErrUserInfo
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return ResolvedTarget{}, ErrUnsafeHost
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return ResolvedTarget{}, ErrUnsafeHost
	}

	port, err := effectivePort(u)
	if err != nil {
		return ResolvedTarget{}, err
	}
	if _, ok := v.AllowedPorts[port]; !ok {
		return ResolvedTarget{}, ErrUnsafePort
	}

	var ips []netip.Addr
	if literal, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{literal.Unmap()}
	} else {
		resolver := v.Resolver
		if resolver == nil {
			resolver = NetResolver{}
		}
		ips, err = resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return ResolvedTarget{}, fmt.Errorf("resolve host: %w", err)
		}
		if len(ips) == 0 {
			return ResolvedTarget{}, ErrUnsafeHost
		}
	}

	validated := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		ip = ip.Unmap()
		if !ip.IsValid() || isForbiddenIP(ip) {
			return ResolvedTarget{}, fmt.Errorf("%w: %s", ErrUnsafeIP, ip)
		}
		validated = append(validated, ip)
	}

	return ResolvedTarget{URL: u, Host: host, Port: port, IPs: validated}, nil
}

func effectivePort(u *url.URL) (uint16, error) {
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return 0, ErrUnsafePort
		}
		return uint16(n), nil
	}
	if u.Scheme == "https" {
		return 443, nil
	}
	return 80, nil
}

var forbiddenPrefixes = mustPrefixes(
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
	"2001:db8::/32",
)

func isForbiddenIP(ip netip.Addr) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, p := range forbiddenPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func mustPrefixes(values ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	for _, s := range values {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}
