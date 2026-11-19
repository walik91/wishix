package productimport

import (
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// MaximumAdapter recognizes only the store's product description frame.
// It supplies an inert, tightly validated candidate; the importer performs the fetch.
type MaximumAdapter struct{}

var maximumProductPath = regexp.MustCompile(`^/(ru|ro)/[0-9]+/?$`)
var maximumModelCode = regexp.MustCompile(`^[0-9]{1,16}$`)

func (MaximumAdapter) Match(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return (host == "maximum.md" || host == "www.maximum.md") && maximumProductPath.MatchString(u.Path)
}

func hasToken(value, token string) bool {
	for _, part := range strings.Fields(value) {
		if part == token {
			return true
		}
	}
	return false
}

func (MaximumAdapter) Enrich(d Document, m Metadata) Metadata {
	if m.NotProduct || m.Description != "" || !maximumProductPath.MatchString(d.URL.Path) {
		return m
	}
	var codes, frames []string
	count := 0
	tooDeep := false
	var walk func(*html.Node, bool, int)
	walk = func(n *html.Node, inDescription bool, depth int) {
		count++
		if count > 100000 {
			return
		}
		if depth > 256 {
			tooDeep = true
			return
		}
		if n.Type == html.ElementNode {
			if n.Data == "span" && hasToken(attr(n, "class"), "product-code-value") {
				codes = append(codes, nodeText(n))
			}
			if n.Data == "article" && hasToken(attr(n, "class"), "product-view-description") && hasToken(attr(n, "itemprop"), "description") {
				inDescription = true
			}
			if inDescription && n.Data == "iframe" && hasToken(attr(n, "class"), "mx-richcontent-autosize") {
				frames = append(frames, attr(n, "src"))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inDescription, depth+1)
		}
	}
	walk(d.HTML, false, 0)
	if count > 100000 || tooDeep || len(codes) != 1 || len(frames) != 1 || !maximumModelCode.MatchString(codes[0]) {
		return m
	}
	u, err := url.Parse(frames[0])
	if err != nil || u.Scheme != "https" || u.Host != "plus.maximum.md" || u.User != nil || u.Fragment != "" || u.Path != "/RichContentViewer/Models/"+codes[0]+"/index.html" || u.RawPath != "" {
		return m
	}
	query, err := url.ParseQuery(u.RawQuery)
	lang := maximumProductPath.FindStringSubmatch(d.URL.Path)[1]
	if err != nil || len(query) != 1 || len(query["culture"]) != 1 || query.Get("culture") != lang {
		return m
	}
	m.descriptionResource = &descriptionResource{url: u.String(), extract: maximumDescription}
	return m
}

func maximumDescription(raw []byte) string {
	_, _, certain := charset.DetermineEncoding(raw, "")
	var reader io.Reader = bytes.NewReader(raw)
	var err error
	if certain || !utf8.Valid(raw) {
		reader, err = charset.NewReader(reader, "")
	}
	if err != nil {
		return ""
	}
	doc, err := html.Parse(reader)
	if err != nil {
		return ""
	}
	var main []*html.Node
	count := 0
	tooDeep := false
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		count++
		if count > 100000 {
			return
		}
		if depth > 256 {
			tooDeep = true
			return
		}
		if n.Type == html.ElementNode && n.Data == "main" && attr(n, "role") == "main" {
			main = append(main, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, depth+1)
		}
	}
	walk(doc, 0)
	if count > 100000 || tooDeep || len(main) != 1 {
		return ""
	}
	description := nodeTextLimit(main[0], 5001)
	if runes := []rune(description); len(runes) > 5000 {
		clipped := string(runes[:4999])
		if space := strings.LastIndex(clipped, " "); space > len(clipped)/2 {
			clipped = clipped[:space]
		}
		return strings.TrimSpace(clipped) + "…"
	}
	return description
}
