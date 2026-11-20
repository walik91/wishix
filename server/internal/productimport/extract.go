package productimport

import (
	"bytes"
	"encoding/json"
	stdhtml "html"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"wishix/server/internal/wishlimits"
)

// Document is bounded, inert HTML. Adapters may inspect it but do not fetch URLs.
type Document struct {
	URL  *url.URL
	HTML *html.Node
}

type Metadata struct {
	Title               string
	Description         string
	Price               string
	Currency            string
	ShopName            string
	Images              []string
	NotProduct          bool
	descriptionResource *descriptionResource
}

type descriptionResource struct {
	url     string
	extract func([]byte) string
}

// Adapter is an optional override for a confirmed site-specific incompatibility.
// The default registry is empty. Image URLs still pass the common secure fetcher.
type Adapter interface {
	Match(*url.URL) bool
	Enrich(Document, Metadata) Metadata
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func content(n *html.Node) string {
	if v := attr(n, "content"); v != "" {
		return v
	}
	if v := attr(n, "href"); v != "" {
		return v
	}
	if v := attr(n, "src"); v != "" {
		return v
	}
	return nodeText(n)
}
func nodeText(n *html.Node) string {
	return nodeTextLimit(n, 1000)
}
func nodeTextLimit(n *html.Node, limit int) string {
	var b strings.Builder
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		if depth > 64 || b.Len() > 24000 {
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "iframe", "object", "embed", "template", "noscript", "svg", "canvas":
				return
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, depth+1)
		}
	}
	walk(n, 0)
	return clean(b.String(), limit)
}

// Imported text can contain raw or entity-escaped markup. Decode bounded layers
// before parsing it inertly; never load embedded resources or execute scripts.
func plainText(s string, limit int) string {
	if len(s) > 50000 {
		s = s[:50000]
	}
	for i := 0; i < 4; i++ {
		decoded := stdhtml.UnescapeString(s)
		if decoded == s {
			break
		}
		s = decoded
	}
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return ""
	}
	return nodeTextLimit(doc, limit)
}
func clean(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > limit {
		s = string(runes[:limit])
	}
	return s
}

func limitDescription(s string) string {
	runes := []rune(s)
	if len(runes) <= wishlimits.Description {
		return s
	}
	clipped := string(runes[:wishlimits.Description-1])
	if space := strings.LastIndex(clipped, " "); space > len(clipped)/2 {
		clipped = clipped[:space]
	}
	return strings.TrimSpace(clipped) + "…"
}
func types(v any, wanted string) bool {
	switch v := v.(type) {
	case string:
		for _, token := range strings.Fields(v) {
			token = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(token, "https://schema.org/"), "http://schema.org/"), "//schema.org/"), "schema:")
			if strings.EqualFold(strings.TrimSuffix(token, "/"), wanted) {
				return true
			}
		}
	case []any:
		for _, value := range v {
			if types(value, wanted) {
				return true
			}
		}
	}
	return false
}
func str(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return string(v)
	}
	return ""
}
func objects(v any) []map[string]any {
	if m, ok := v.(map[string]any); ok {
		return []map[string]any{m}
	}
	var result []map[string]any
	if values, ok := v.([]any); ok {
		for _, v := range values {
			if m, ok := v.(map[string]any); ok {
				result = append(result, m)
			}
		}
	}
	return result
}
func decodeJSON(raw string) any {
	if len(raw) > 512*1024 {
		return nil
	}
	// Bound nesting before decoding/traversal (the document itself is also capped).
	depth, quoted, escape := 0, false, false
	for _, ch := range raw {
		if quoted {
			if escape {
				escape = false
			} else if ch == '\\' {
				escape = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
		} else if ch == '[' || ch == '{' {
			depth++
			if depth > 32 {
				return nil
			}
		} else if ch == ']' || ch == '}' {
			depth--
		}
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		return nil
	}
	return v
}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,9})(\.[0-9]+)?$`)
var groupedPrice = regexp.MustCompile(`^[0-9]{1,3}([ ,\.][0-9]{3})+([,.][0-9]{1,2})?$`)

// decimal preserves exact values; no floating-point conversion or rounding.
func decimal(raw string) string {
	s := strings.TrimSpace(strings.ReplaceAll(raw, "\u00a0", " "))
	if strings.Contains(s, ",") || strings.Contains(s, " ") {
		if groupedPrice.MatchString(s) {
			last := strings.LastIndexAny(s, ",.")
			fraction := ""
			if last >= 0 && len(s)-last-1 <= 2 {
				fraction = "." + s[last+1:]
				s = s[:last]
			}
			s = strings.NewReplacer(",", "", ".", "", " ", "").Replace(s) + fraction
		} else if strings.Count(s, ",") == 1 && !strings.Contains(s, ".") && !strings.Contains(s, " ") {
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			return ""
		}
	}
	if !decimalPattern.MatchString(s) {
		return ""
	}
	parts := strings.SplitN(s, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if len(fraction) > 2 {
		return ""
	}
	return parts[0] + "." + (fraction + "00")[:2]
}

func samePage(raw string, base *url.URL) bool {
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		return false
	}
	u = base.ResolveReference(u)
	queryWithoutTracking := func(u *url.URL) (string, bool) {
		values, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return "", false
		}
		for key := range values {
			if strings.HasPrefix(strings.ToLower(key), "utm_") {
				delete(values, key)
			}
		}
		return values.Encode(), true
	}
	uq, validU := queryWithoutTracking(u)
	bq, validBase := queryWithoutTracking(base)
	return validU && validBase && strings.EqualFold(u.Host, base.Host) && strings.TrimRight(u.Path, "/") == strings.TrimRight(base.Path, "/") && uq == bq
}

func imageValues(v any) []string {
	var result []string
	switch v := v.(type) {
	case string:
		result = append(result, v)
	case []any:
		for _, v := range v {
			result = append(result, imageValues(v)...)
			if len(result) >= 24 {
				break
			}
		}
	case map[string]any:
		for _, k := range []string{"contentUrl", "url", "@id"} {
			if s := str(v[k]); s != "" {
				result = append(result, s)
			}
		}
	}
	return result
}

func priceFrom(offers any, page *url.URL) (string, string) {
	var valid []map[string]any
	for _, offer := range objects(offers) {
		if types(offer["@type"], "AggregateOffer") {
			if str(offer["lowPrice"]) != str(offer["highPrice"]) {
				continue
			}
			offer["price"] = offer["lowPrice"]
		}
		valid = append(valid, offer)
	}
	// Prefer a current-page offer among variants; ambiguous differing prices are omitted.
	var matched []map[string]any
	for _, o := range valid {
		if samePage(str(o["url"]), page) {
			matched = append(matched, o)
		}
	}
	if len(matched) > 0 {
		valid = matched
	}
	price, currency := "", ""
	for _, o := range valid {
		p, c := decimal(plainText(str(o["price"]), 100)), strings.ToUpper(plainText(str(o["priceCurrency"]), 100))
		if p == "" {
			for _, spec := range objects(o["priceSpecification"]) {
				p, c = decimal(plainText(str(spec["price"]), 100)), strings.ToUpper(plainText(str(spec["priceCurrency"]), 100))
				if p != "" {
					break
				}
			}
		}
		if p == "" || !currencyPattern.MatchString(c) {
			continue
		}
		if price != "" && (price != p || currency != c) {
			return "", ""
		}
		price, currency = p, c
	}
	return price, currency
}

func extract(raw []byte, page *url.URL, adapters []Adapter) (Metadata, error) {
	// HTML without an explicit charset is often UTF-8; do not misdecode it as Windows-1252.
	_, _, certain := charset.DetermineEncoding(raw, "")
	var reader io.Reader = bytes.NewReader(raw)
	var err error
	if certain || !utf8.Valid(raw) {
		reader, err = charset.NewReader(reader, "")
	}
	if err != nil {
		return Metadata{}, failure(ErrFetch, "charset_invalid", 0)
	}
	doc, err := html.Parse(reader)
	if err != nil {
		return Metadata{}, ErrFetch
	}
	meta := map[string]string{}
	var h1 []string
	var products []map[string]any
	var micro []*html.Node
	var galleries []string
	var descriptiveImages []*html.Node
	collection, cards, count := false, 0, 0
	pageTitle := ""
	var jsonWalk func(any, int)
	jsonWalk = func(v any, depth int) {
		if depth > 8 || len(products) > 64 {
			return
		}
		if a, ok := v.([]any); ok {
			for _, e := range a {
				jsonWalk(e, depth+1)
			}
			return
		}
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		if types(m["@type"], "CollectionPage") || types(m["@type"], "ItemList") {
			collection = true
			return
		}
		if types(m["@type"], "Product") {
			products = append(products, m)
			return
		}
		if types(m["@type"], "ProductGroup") {
			jsonWalk(m["hasVariant"], depth+1)
			return
		}
		for _, key := range []string{"@graph", "mainEntity", "mainEntityOfPage"} {
			jsonWalk(m[key], depth+1)
		}
	}
	var galleryJSON func(any, int)
	galleryJSON = func(v any, depth int) {
		if depth > 5 || len(galleries) >= 24 {
			return
		}
		if m, ok := v.(map[string]any); ok {
			// Only image/gallery fields, never execute embedded JavaScript or traverse arbitrary scripts.
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				lower := strings.ToLower(k)
				if strings.Contains(lower, "image") || strings.Contains(lower, "photo") || strings.Contains(lower, "gallery") {
					galleries = append(galleries, imageValues(m[k])...)
				}
			}
		}
	}
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		count++
		if count > 100000 || depth > 256 {
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				key := strings.ToLower(attr(n, "property"))
				if key == "" {
					key = strings.ToLower(attr(n, "name"))
				}
				if len(key) < 100 && meta[key] == "" {
					meta[key] = attr(n, "content")
				}
			case "title":
				pageTitle = nodeText(n)
			case "h1":
				value := nodeText(n)
				if value != "" && len(h1) < 8 {
					h1 = append(h1, value)
				}
			case "script":
				if strings.EqualFold(strings.TrimSpace(strings.Split(attr(n, "type"), ";")[0]), "application/ld+json") && n.FirstChild != nil {
					jsonWalk(decodeJSON(n.FirstChild.Data), 0)
				}
			case "link":
				if strings.EqualFold(attr(n, "rel"), "image_src") {
					galleries = append(galleries, attr(n, "href"))
				}
			case "img":
				if len(descriptiveImages) < 512 && len([]rune(attr(n, "alt"))) >= 20 && attr(n, "src") != "" {
					descriptiveImages = append(descriptiveImages, n)
				}
			}
			if types(attr(n, "itemtype"), "Product") || types(attr(n, "typeof"), "Product") {
				micro = append(micro, n)
			}
			if attr(n, "data-product-id") != "" {
				cards++
			}
			for _, a := range n.Attr {
				key := strings.ToLower(a.Key)
				if strings.Contains(key, "product") || strings.Contains(key, "gallery") || key == ":imgs" || key == "data-images" {
					v := decodeJSON(a.Val)
					if key == ":imgs" || key == "data-images" {
						galleries = append(galleries, imageValues(v)...)
					} else {
						galleryJSON(v, 0)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, depth+1)
		}
	}
	walk(doc, 0)
	if count > 100000 {
		return Metadata{}, failure(ErrFetch, "html_too_complex", 0)
	}
	result := Metadata{ShopName: plainText(meta["og:site_name"], 120)}
	var product map[string]any
	bestScore, tied := 0, false
	for _, p := range products {
		score := 0
		if samePage(str(p["url"]), page) || samePage(str(p["@id"]), page) {
			score = 100
		}
		for _, offer := range objects(p["offers"]) {
			if samePage(str(offer["url"]), page) && score < 90 {
				score = 90
			}
		}
		name := plainText(str(p["name"]), 200)
		if name != "" && score == 0 {
			for _, heading := range h1 {
				if strings.EqualFold(name, heading) {
					score = 50
					break
				}
			}
		}
		if score > bestScore {
			product, bestScore, tied = p, score, false
		} else if score == bestScore && score > 0 {
			tied = true
		}
	}
	if tied {
		product = nil
	}
	if product == nil && len(products) == 1 && !collection {
		product = products[0]
	}
	if product != nil {
		result.Title = str(product["name"])
		result.Description = plainText(str(product["description"]), wishlimits.Description+1)
		result.Price, result.Currency = priceFrom(product["offers"], page)
		result.Images = append(result.Images, imageValues(product["image"])...)
	}
	if product == nil && len(micro) == 1 && !collection {
		var scope func(*html.Node)
		scope = func(n *html.Node) {
			if n != micro[0] && (types(attr(n, "itemtype"), "Product") || types(attr(n, "typeof"), "Product")) {
				return
			}
			for _, prop := range strings.Fields(attr(n, "itemprop") + " " + attr(n, "property")) {
				prop = strings.TrimPrefix(prop, "schema:")
				switch prop {
				case "name":
					if result.Title == "" {
						result.Title = content(n)
					}
				case "description":
					if result.Description == "" {
						if v := attr(n, "content"); v != "" {
							result.Description = plainText(v, wishlimits.Description+1)
						} else {
							result.Description = nodeTextLimit(n, wishlimits.Description+1)
						}
					}
				case "price":
					if result.Price == "" {
						result.Price = decimal(plainText(content(n), 100))
					}
				case "priceCurrency":
					if result.Currency == "" {
						result.Currency = strings.ToUpper(plainText(content(n), 100))
					}
				case "image":
					result.Images = append(result.Images, content(n))
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				scope(c)
			}
		}
		scope(micro[0])
	}
	// Collection pages must not import logos, card prices, or arbitrary recommendations.
	result.NotProduct = product == nil && (collection || len(products) > 1 || len(micro) > 1 || cards >= 4)
	if !result.NotProduct {
		ogTitle := meta["og:title"]
		if ogTitle == "" {
			ogTitle = meta["twitter:title"]
		}
		mainTitle := ""
		if len(h1) > 0 {
			mainTitle = h1[0]
			for _, h := range h1 {
				if h != mainTitle {
					mainTitle = ""
					break
				}
			}
		}
		// Multiple section headings are common. A single heading contained in the
		// document title is a safer main-heading signal than an arbitrary first H1.
		if mainTitle == "" && pageTitle != "" {
			for _, heading := range h1 {
				if strings.Contains(strings.ToLower(pageTitle), strings.ToLower(heading)) {
					if mainTitle != "" && !strings.EqualFold(mainTitle, heading) {
						mainTitle = ""
						break
					}
					mainTitle = heading
				}
			}
		}
		// Prefer a matching main heading: keeps the selected variant and drops schema suffix noise.
		if mainTitle != "" && (result.Title == "" || strings.HasPrefix(strings.ToLower(mainTitle), strings.ToLower(result.Title)) || (ogTitle == mainTitle && strings.HasPrefix(strings.ToLower(result.Title), strings.ToLower(mainTitle)))) {
			result.Title = mainTitle
		}
		if result.Title == "" {
			result.Title = ogTitle
		}
		if result.Title == "" {
			result.Title = pageTitle
		}
		if result.Price == "" {
			for _, prefix := range []string{"product:price:", "og:price:"} {
				p, c := decimal(plainText(meta[prefix+"amount"], 100)), strings.ToUpper(plainText(meta[prefix+"currency"], 100))
				if p != "" && currencyPattern.MatchString(c) {
					result.Price, result.Currency = p, c
					break
				}
			}
		}
		result.Images = append(result.Images, meta["og:image"], meta["og:image:secure_url"], meta["twitter:image"], meta["twitter:image:src"])
		result.Images = append(result.Images, galleries...)
		// Fall back to an image whose descriptive alt starts with the selected
		// product title; avoid guessing from unrelated images or short names.
		if len([]rune(result.Title)) >= 20 {
			for _, img := range descriptiveImages {
				alt := clean(attr(img, "alt"), 1000)
				if strings.HasPrefix(strings.ToLower(alt), strings.ToLower(result.Title)) {
					result.Images = append(result.Images, attr(img, "src"))
				}
			}
		}
	}
	for _, adapter := range adapters {
		if adapter.Match(page) {
			result = adapter.Enrich(Document{URL: page, HTML: doc}, result)
			break
		}
	}
	if result.NotProduct {
		return Metadata{NotProduct: true}, nil
	}
	result.Title = plainText(result.Title, 200)
	result.Description = limitDescription(plainText(result.Description, wishlimits.Description+1))
	result.ShopName = plainText(result.ShopName, 120)
	result.Price = decimal(plainText(result.Price, 100))
	result.Currency = strings.ToUpper(plainText(result.Currency, 100))
	if result.Price == "" || !currencyPattern.MatchString(result.Currency) {
		result.Price, result.Currency = "", ""
	}
	var images []string
	seen := map[string]bool{}
	for _, raw := range result.Images {
		if len(images) >= 24 {
			break
		}
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || raw == "" {
			continue
		}
		resolved, err := ValidateURL(page.ResolveReference(u).String())
		if err != nil || seen[resolved.String()] {
			continue
		}
		seen[resolved.String()] = true
		images = append(images, resolved.String())
	}
	result.Images = images
	return result, nil
}
