// Package productimport imports untrusted public product pages without executing them.
package productimport

import (
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/idna"
)

var (
	ErrURL   = errors.New("import_url_invalid")
	ErrFetch = errors.New("import_unavailable")
	ErrBusy  = errors.New("import_busy")
)

const MaxHTML = 2 * 1024 * 1024

// Explicitly exclude special-purpose ranges as well as IsPrivate/IsGlobalUnicast.
// IPv6 is restricted to ordinary global unicast, excluding transition mechanisms.
var denied = func() []netip.Prefix {
	var result []netip.Prefix
	for _, raw := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/3",
		"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20",
	} {
		result = append(result, netip.MustParsePrefix(raw))
	}
	return result
}()

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range denied {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

// ValidateURL is also applied to redirects and every image candidate.
func ValidateURL(raw string) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return nil, ErrURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return nil, ErrURL
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if strings.Contains(host, "%") {
		return nil, ErrURL
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicIP(ip) {
			return nil, ErrURL
		}
		host = ip.String()
	} else {
		host, err = idna.Lookup.ToASCII(host)
		if err != nil || !strings.Contains(host, ".") || len(host) > 253 {
			return nil, ErrURL
		}
		for _, suffix := range []string{".localhost", ".local", ".internal", ".test", ".invalid", ".example", ".onion"} {
			if strings.HasSuffix(host, suffix) {
				return nil, ErrURL
			}
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return nil, ErrURL
			}
			for _, ch := range label {
				if ch != '-' && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
					return nil, ErrURL
				}
			}
		}
	}
	port := u.Port()
	if port != "" && !((u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443")) {
		return nil, ErrURL
	}
	u.Host = host
	if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	}
	u.Fragment = ""
	return u, nil
}

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type dialFunc func(context.Context, string, string) (net.Conn, error)

type fetcher struct{ client *http.Client }

func newFetcher() *fetcher {
	return secureFetcher(net.DefaultResolver, (&net.Dialer{Timeout: 4 * time.Second}).DialContext)
}

func secureFetcher(dns resolver, dial dialFunc) *fetcher {
	transport := &http.Transport{
		Proxy:             nil, // Never inherit proxy environment variables or internal proxy access.
		DisableKeepAlives: true, MaxResponseHeaderBytes: 32 * 1024,
		TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 6 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || (port != "80" && port != "443") {
				return nil, ErrURL
			}
			ips, err := dns.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 || len(ips) > 32 {
				return nil, ErrFetch
			}
			// Reject mixed public/private DNS answers before trying ANY address.
			for _, ip := range ips {
				if !publicIP(ip) {
					return nil, ErrURL
				}
			}
			for _, ip := range ips {
				// Pin the checked numeric address. TLS still verifies the original host.
				conn, err := dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
				if err == nil {
					return conn, nil
				}
				if ctx.Err() != nil {
					break
				}
			}
			return nil, ErrFetch
		},
	}
	return &fetcher{client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (f *fetcher) get(ctx context.Context, raw, locale string, max int64, image bool) ([]byte, *url.URL, error) {
	u, err := ValidateURL(raw)
	if err != nil {
		return nil, nil, err
	}
	for hop := 0; hop <= 3; hop++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("User-Agent", "Wishix/0.1 (product preview)")
		req.Header.Set("Accept-Language", locale+",en;q=0.8")
		if image {
			req.Header.Set("Accept", "image/jpeg,image/png,image/webp")
		} else {
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
		}
		res, err := f.client.Do(req)
		if err != nil {
			return nil, nil, ErrFetch
		} // Do not expose URL-bearing network errors.
		if res.StatusCode == 301 || res.StatusCode == 302 || res.StatusCode == 303 || res.StatusCode == 307 || res.StatusCode == 308 {
			location := res.Header.Get("Location")
			res.Body.Close()
			if hop == 3 || location == "" {
				return nil, nil, ErrFetch
			}
			next, parseErr := url.Parse(location)
			if parseErr != nil {
				return nil, nil, ErrFetch
			}
			u, err = ValidateURL(u.ResolveReference(next).String())
			if err != nil {
				return nil, nil, err
			}
			continue
		}
		media, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
		allowed := media == "text/html" || media == "application/xhtml+xml"
		if image {
			allowed = media == "image/jpeg" || media == "image/png" || media == "image/webp" || media == "application/octet-stream"
		}
		if res.StatusCode != 200 || !allowed || res.ContentLength > max {
			res.Body.Close()
			return nil, nil, ErrFetch
		}
		// net/http transparently decompresses gzip; the limit applies to decoded bytes.
		body, readErr := io.ReadAll(io.LimitReader(res.Body, max+1))
		res.Body.Close()
		if readErr != nil || len(body) == 0 || int64(len(body)) > max {
			return nil, nil, ErrFetch
		}
		return body, u, nil
	}
	return nil, nil, ErrFetch
}
