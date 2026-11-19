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

const MaxHTML = 4 * 1024 * 1024

// Failure contains only fixed diagnostic codes and a status, never upstream URLs or bodies.
type Failure struct {
	Cause      error
	Reason     string
	HTTPStatus int
}

func (f *Failure) Error() string { return f.Cause.Error() }
func (f *Failure) Unwrap() error { return f.Cause }

func failure(cause error, reason string, status int) error {
	return &Failure{Cause: cause, Reason: reason, HTTPStatus: status}
}

func FailureDetails(err error) (string, int) {
	var f *Failure
	if errors.As(err, &f) {
		return f.Reason, f.HTTPStatus
	}
	switch {
	case errors.Is(err, ErrURL):
		return "invalid_url", 0
	case errors.Is(err, ErrBusy):
		return "busy", 0
	case errors.Is(err, context.Canceled):
		return "cancelled", 0
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", 0
	case errors.Is(err, ErrFetch):
		return "unavailable", 0
	default:
		return "internal_error", 0
	}
}

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
				return nil, failure(ErrFetch, "dns_failed", 0)
			}
			// Reject mixed public/private DNS answers before trying ANY address.
			for _, ip := range ips {
				if !publicIP(ip) {
					return nil, failure(ErrURL, "address_forbidden", 0)
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
			return nil, failure(ErrFetch, "connect_failed", 0)
		},
	}
	return &fetcher{client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (f *fetcher) get(ctx context.Context, raw, locale string, max int64, image bool) ([]byte, *url.URL, error) {
	return f.getWithRedirects(ctx, raw, locale, max, image, true)
}

// Extra description resources cannot redirect, even to another public host.
func (f *fetcher) getWithRedirects(ctx context.Context, raw, locale string, max int64, image, allowRedirects bool) ([]byte, *url.URL, error) {
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
			if ctx.Err() != nil {
				return nil, nil, failure(ErrFetch, "cancelled_or_timeout", 0)
			}
			var detail *Failure
			if errors.As(err, &detail) {
				return nil, nil, detail
			}
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				return nil, nil, failure(ErrFetch, "timeout", 0)
			}
			return nil, nil, failure(ErrFetch, "request_failed", 0)
		} // Do not expose URL-bearing network errors.
		if res.StatusCode == 301 || res.StatusCode == 302 || res.StatusCode == 303 || res.StatusCode == 307 || res.StatusCode == 308 {
			location := res.Header.Get("Location")
			res.Body.Close()
			if !allowRedirects {
				return nil, nil, failure(ErrFetch, "redirect_forbidden", res.StatusCode)
			}
			if hop == 3 || location == "" {
				return nil, nil, failure(ErrFetch, "redirect_limit_or_missing_location", res.StatusCode)
			}
			next, parseErr := url.Parse(location)
			if parseErr != nil {
				return nil, nil, failure(ErrFetch, "invalid_redirect", res.StatusCode)
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
		if res.StatusCode != 200 {
			res.Body.Close()
			return nil, nil, failure(ErrFetch, "http_status", res.StatusCode)
		}
		if !allowed {
			res.Body.Close()
			return nil, nil, failure(ErrFetch, "unsupported_content_type", res.StatusCode)
		}
		if res.ContentLength > max {
			res.Body.Close()
			return nil, nil, failure(ErrFetch, "response_too_large", res.StatusCode)
		}
		// net/http transparently decompresses gzip; the limit applies to decoded bytes.
		body, readErr := io.ReadAll(io.LimitReader(res.Body, max+1))
		res.Body.Close()
		if readErr != nil {
			return nil, nil, failure(ErrFetch, "read_failed", res.StatusCode)
		}
		if int64(len(body)) > max {
			return nil, nil, failure(ErrFetch, "response_too_large", res.StatusCode)
		}
		if len(body) == 0 {
			return nil, nil, failure(ErrFetch, "empty_response", res.StatusCode)
		}
		return body, u, nil
	}
	return nil, nil, ErrFetch
}
