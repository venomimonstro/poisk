package fetcher

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
)

func validatedDialContext(validator TargetValidator, timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		scheme := "http"
		if port == "443" {
			scheme = "https"
		} else if port != "80" {
			return nil, fmt.Errorf("unsupported outbound port %s", port)
		}

		target, err := validator.Validate(ctx, scheme+"://"+net.JoinHostPort(host, port)+"/")
		if err != nil {
			return nil, err
		}
		for _, ip := range target.IPs {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), strconv.Itoa(int(target.Port))))
			if dialErr == nil {
				return conn, nil
			}
			err = dialErr
		}
		if err == nil {
			err = fmt.Errorf("no validated address for %s", host)
		}
		return nil, err
	}
}

var _ = crawlersecurity.ResolvedTarget{}
