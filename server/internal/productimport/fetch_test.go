package productimport

import (
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type fixedDNS []netip.Addr

func (d fixedDNS) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) { return d, nil }

type rebindingDNS struct{ calls int }

func (d *rebindingDNS) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	d.calls++
	if d.calls == 1 {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
}

func TestDNSRebindingOnRedirectIsRejected(t *testing.T) {
	dns := &rebindingDNS{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/next")
		w.WriteHeader(302)
	}))
	defer server.Close()
	dials := 0
	f := secureFetcher(dns, func(ctx context.Context, network, address string) (net.Conn, error) {
		dials++
		if address != "8.8.8.8:80" {
			t.Error("unchecked dial address", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	_, _, err := f.get(context.Background(), "http://shop.example.com/item", "en", 1000, false)
	if err == nil || dns.calls != 2 || dials != 1 {
		t.Fatalf("rebound DNS was not rejected: %v, lookups=%d dials=%d", err, dns.calls, dials)
	}
}

func TestURLAndAddressPolicy(t *testing.T) {
	for _, raw := range []string{
		"http://localhost/item", "http://127.0.0.1/", "http://[::1]/", "http://[::ffff:127.0.0.1]/",
		"http://10.0.0.1/", "http://169.254.169.254/", "http://100.100.100.200/", "http://192.168.0.1/",
		"http://[64:ff9b::7f00:1]/", "http://[2002:7f00:1::]/", "http://[2001:db8::1]/",
		"https://shop.example.com:8080/", "http://shop.example.com:443/", "https://user:secret@shop.example.com/",
		"file:///etc/passwd", "ftp://shop.example.com/", "http://2130706433/", "http://shop.local/", "http://metadata.google.internal/",
		"http://shop.example.com/%0a\n", "http://[fe80::1%25eth0]/", "https://shop.example.com\\@127.0.0.1/",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ValidateURL(raw); err == nil {
				t.Fatal("accepted unsafe URL")
			}
		})
	}
	for _, raw := range []string{"https://shop.example.com/item?q=1#image", "https://магазин.md/item", "http://8.8.8.8/", "https://[2606:4700:4700::1111]/"} {
		if _, err := ValidateURL(raw); err != nil {
			t.Errorf("rejected public URL %s", raw)
		}
	}
	for _, raw := range []string{"0.0.0.0", "192.0.0.8", "198.19.0.1", "192.0.2.1", "240.0.0.1", "ff02::1", "fc00::1", "2001::1", "3fff::1"} {
		if publicIP(netip.MustParseAddr(raw)) {
			t.Errorf("accepted special address %s", raw)
		}
	}
}

func testFetcher(t *testing.T, handler http.HandlerFunc) (*fetcher, *[]string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	var dialed []string
	f := secureFetcher(fixedDNS{netip.MustParseAddr("8.8.8.8")}, func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	return f, &dialed
}

func TestFetchPinsDNSAndChecksRedirects(t *testing.T) {
	var privateRequests int
	f, dialed := testFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("forwarded credentials")
		}
		if r.Host != "shop.example.com" {
			privateRequests++
		}
		w.Header().Set("Location", "http://127.0.0.1/private")
		w.WriteHeader(302)
	})
	_, _, err := f.get(context.Background(), "http://shop.example.com/item", "en", 1000, false)
	if !errors.Is(err, ErrURL) || len(*dialed) != 1 || (*dialed)[0] != "8.8.8.8:80" || privateRequests != 0 {
		t.Fatalf("redirect policy failed: %v %v", err, *dialed)
	}
	if f.client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("environment proxy enabled")
	}
}

func TestMixedDNSNeverDials(t *testing.T) {
	called := false
	f := secureFetcher(fixedDNS{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, errors.New("unexpected")
	})
	if _, _, err := f.get(context.Background(), "http://shop.example.com/", "en", 1000, false); err == nil || called {
		t.Fatal("mixed DNS allowed a connection")
	}
}

func TestFetchBounds(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		limit   int64
		image   bool
	}{
		{"gzip expansion", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Encoding", "gzip")
			z := gzip.NewWriter(w)
			_, _ = z.Write([]byte(strings.Repeat("a", 2000)))
			_ = z.Close()
		}, 1000, false},
		{"redirect loop", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Location", "/loop"); w.WriteHeader(302) }, 1000, false},
		{"bad content type", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte("alert(1)"))
		}, 1000, false},
		{"html pretending image", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("bad"))
		}, 1000, true},
		{"oversized", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(strings.Repeat("a", 2000)))
		}, 1000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, dialed := testFetcher(t, tt.handler)
			if _, _, err := f.get(context.Background(), "http://shop.example.com/", "en", tt.limit, tt.image); err == nil {
				t.Fatal("accepted invalid response")
			}
			if len(*dialed) > 4 {
				t.Fatal("unbounded redirects")
			}
		})
	}
	f, _ := testFetcher(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := f.get(ctx, "http://shop.example.com/", "en", 1000, false)
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("request not cancelled")
	}
}
