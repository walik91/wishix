package productimport

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

const maximumPageFixture = `<title>Dyson vacuum</title><span class="product-code-value">332146</span><article class="product-view-description" itemprop="description"><iframe class="mx-richcontent-autosize" src="https://plus.maximum.md/RichContentViewer/Models/332146/index.html?culture=ro"></iframe></article>`

func TestMaximumAdapterMatchesOnlyIdentifiedProductDescription(t *testing.T) {
	for _, tt := range []struct {
		name, page, html string
		want             bool
	}{
		{"product", "https://maximum.md/ro/6965529/", maximumPageFixture, true},
		{"www and query", "https://www.maximum.md/ro/6965529/?tracking=1", maximumPageFixture, true},
		{"other host", "https://other.md/ro/6965529/", maximumPageFixture, false},
		{"lookalike host", "https://maximum.md.other.md/ro/6965529/", maximumPageFixture, false},
		{"category", "https://maximum.md/ro/vacuums/", maximumPageFixture, false},
		{"mismatched model", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "/332146/index.html", "/1/index.html", 1), false},
		{"missing model", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "product-code-value", "other", 1), false},
		{"ambiguous model", "https://maximum.md/ro/6965529/", maximumPageFixture + `<span class="product-code-value">332146</span>`, false},
		{"wrong frame class", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "mx-richcontent-autosize", "other", 1), false},
		{"other frame host", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "plus.maximum.md", "evil.example.com", 1), false},
		{"userinfo", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "plus.maximum.md", "user@plus.maximum.md", 1), false},
		{"port", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "plus.maximum.md", "plus.maximum.md:443", 1), false},
		{"http frame", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "https://plus", "http://plus", 1), false},
		{"wrong language", "https://maximum.md/ru/6965529/", maximumPageFixture, false},
		{"extra parameter", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "culture=ro", "culture=ro&amp;url=bad", 1), false},
		{"fragment", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "culture=ro", "culture=ro#fragment", 1), false},
		{"frame outside description", "https://maximum.md/ro/6965529/", strings.Replace(maximumPageFixture, "product-view-description", "other", 1), false},
		{"existing product description", "https://maximum.md/ro/6965529/", maximumPageFixture + `<script type="application/ld+json">{"@type":"Product","name":"Dyson vacuum","description":"Already available"}</script>`, false},
		{"excessive nesting", "https://maximum.md/ro/6965529/", maximumPageFixture + strings.Repeat("<div>", 260) + "text" + strings.Repeat("</div>", 260), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			page, _ := url.Parse(tt.page)
			m, err := extract([]byte(tt.html), page, []Adapter{MaximumAdapter{}})
			if err != nil || (m.descriptionResource != nil) != tt.want {
				t.Fatalf("description candidate = %v, err = %v", m.descriptionResource != nil, err)
			}
		})
	}
}

type maximumRoundTripper func(*http.Request) (*http.Response, error)

func (f maximumRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMaximumPreviewUsesBoundedDescriptionResource(t *testing.T) {
	for _, mode := range []string{"success", "missing", "redirect_private", "redirect_public", "oversized", "oversized_stream", "ambiguous_main"} {
		t.Run(mode, func(t *testing.T) {
			var requests []string
			i := New(MaximumAdapter{})
			i.fetch.client.Transport = maximumRoundTripper(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Host)
				if r.URL.Host == "maximum.md" {
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(maximumPageFixture)), Request: r}, nil
				}
				if r.URL.Host != "plus.maximum.md" {
					t.Fatal("unexpected third-party or private request")
				}
				resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(`<main role="main"><h1>Useful <b>description</b></h1><script>bad()</script><iframe>bad</iframe><p>Product details</p></main>`)), Request: r}
				switch mode {
				case "missing":
					resp.StatusCode = 404
				case "redirect_private", "redirect_public":
					resp.StatusCode = 302
					if mode == "redirect_private" {
						resp.Header.Set("Location", "http://127.0.0.1/private")
					} else {
						resp.Header.Set("Location", "https://other.example.com/")
					}
				case "oversized":
					resp.ContentLength = 1024*1024 + 1
				case "oversized_stream":
					resp.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", 1024*1024+1)))
				case "ambiguous_main":
					resp.Body = io.NopCloser(strings.NewReader(`<main role="main">One</main><main role="main">Two</main>`))
				}
				return resp, nil
			})
			result, err := i.Preview(context.Background(), "https://maximum.md/ro/6965529/", "ro")
			if err != nil || result.Title != "Dyson vacuum" || len(requests) != 2 {
				t.Fatalf("preview failed: %+v, %v, requests %v", result, err, requests)
			}
			want := ""
			if mode == "success" {
				want = "Useful description Product details"
			}
			if result.Description != want {
				t.Fatalf("description = %q, want %q", result.Description, want)
			}
		})
	}
}

func TestMaximumDescriptionLengthAndMarkup(t *testing.T) {
	description := maximumDescription([]byte(`<main role="main">` + strings.Repeat("текст ", 1100) + `</main>`))
	if len([]rune(description)) > 500 || !strings.HasSuffix(description, "…") || !strings.HasSuffix(description, "текст…") {
		t.Fatalf("description not truncated at a word boundary: %d runes", len([]rune(description)))
	}
	if got := maximumDescription([]byte(`<main role="main"><p>Use &lt;b&gt;plain&lt;/b&gt; text</p><style>hidden</style></main>`)); plainText(got, 5000) != "Use plain text" {
		t.Fatalf("unexpected formatted text: %q", got)
	}
	deep := `<main role="main">` + strings.Repeat("<div>", 260) + "text" + strings.Repeat("</div>", 260) + `</main>`
	if got := maximumDescription([]byte(deep)); got != "" {
		t.Fatalf("accepted excessive nesting: %q", got)
	}
}

func TestMaximumDescriptionCancellationKeepsProduct(t *testing.T) {
	i := New(MaximumAdapter{})
	i.fetch.client.Transport = maximumRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "maximum.md" {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(maximumPageFixture)), Request: r}, nil
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result, err := i.Preview(ctx, "https://maximum.md/ro/6965529/", "ro")
	if err != nil || result.Title != "Dyson vacuum" || result.Description != "" {
		t.Fatalf("cancelled description lost product: %+v, %v", result, err)
	}
}
