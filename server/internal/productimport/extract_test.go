package productimport

import (
	"net/url"
	"strings"
	"testing"
)

func TestExtraction(t *testing.T) {
	page, _ := url.Parse("https://shop.example.com/product")
	tests := []struct {
		name, html, title, price, currency string
		category                           bool
		images                             int
	}{
		{"root product and homepage offer URL", `<h1>Phone Green</h1><script type="application/ld+json">{"@type":"Product","name":"Phone Green","image":["/a.jpg","/a.jpg"],"offers":{"@type":"Offer","url":"https://shop.example.com/","price":"3499.0","priceCurrency":"MDL"}}</script>`, "Phone Green", "3499.00", "MDL", false, 1},
		{"graph selected variant", `<h1>Phone 12 GB / 256 GB</h1><script type="application/ld+json">{"@graph":[{"@type":"BreadcrumbList"},{"@type":["Thing","Product"],"name":"Phone","offers":{"price":32899,"priceCurrency":"MDL"},"image":{"contentUrl":"/phone.webp"}}]}</script>`, "Phone 12 GB / 256 GB", "32899.00", "MDL", false, 1},
		{"clean main heading", `<meta property="og:title" content="Robot White"><h1>Robot White</h1><script type="application/ld+json">{"@type":"Product","name":"Robot White Wet, , ","offers":{"price":4399,"priceCurrency":"MDL"}}</script>`, "Robot White", "4399.00", "MDL", false, 0},
		{"image string and inert gallery JSON", `<script type="application/ld+json">{"@type":"Product","name":"Decoration","image":"/bad.jpg","offers":{"price":"49.95","priceCurrency":"MDL"}}</script><gallery :product='{"all_large_photo_paths":["/good.jpg"]}' :imgs='"/good.jpg"'></gallery>`, "Decoration", "49.95", "MDL", false, 2},
		{"microdata scope", `<div itemscope itemtype="https://schema.org/Product"><span itemprop="name">Dryer Blue</span><meta itemprop="price" content="1699"><meta itemprop="priceCurrency" content="MDL"><img itemprop="image" src="/dryer.webp"></div><div>Monthly payment 99</div>`, "Dryer Blue", "1699.00", "MDL", false, 1},
		{"protocol relative schema microdata", `<h1>Office chair</h1><div itemscope itemtype="//schema.org/Product"><link itemprop="image" href="/chair.webp"><meta itemprop="name" content="Office chair"><span itemprop="description">Chair description</span><div itemprop="offers" itemscope itemtype="//schema.org/Offer"><meta itemprop="price" content="4535.00"><meta itemprop="priceCurrency" content="MDL"></div></div><p>Subscription price 2535 MDL</p>`, "Office chair", "4535.00", "MDL", false, 1},
		{"unrelated schema hostname", `<title>Office chair</title><div itemscope itemtype="//evil.example/schema.org/Product"><meta itemprop="price" content="1"><meta itemprop="priceCurrency" content="MDL"></div>`, "Office chair", "", "", false, 0},
		{"open graph", `<meta property="og:title" content="A &amp; B"><meta property="og:image" content="//cdn.example.com/img.jpg"><meta property="product:price:amount" content="1 699,50"><meta property="product:price:currency" content="mdl">`, "A & B", "1699.50", "MDL", false, 1},
		{"plain title no guessed price", `<title>Handmade item</title><p>999 MDL. Monthly payment 12.</p><script>window.price=999</script>`, "Handmade item", "", "", false, 0},
		{"utf8 metadata without charset", `<meta property="og:title" content="Самоклеящийся держатель"><meta property="og:type" content="product">`, "Самоклеящийся держатель", "", "", false, 0},
		{"main heading among sections", `<title>Shop: Portable Laptop - Details</title><h1>Portable Laptop</h1><h1>Product information</h1>`, "Portable Laptop", "", "", false, 0},
		{"ambiguous headings keep page title", `<title>Shop: Laptop - Details</title><h1>Laptop</h1><h1>Details</h1>`, "Shop: Laptop - Details", "", "", false, 0},
		{"descriptive product image", `<title>Shop: Portable Laptop With Long Battery Life</title><h1>Portable Laptop With Long Battery Life</h1><h1>Details</h1><img alt="Portable Laptop With Long Battery Life, silver" src="/product.jpg"><img alt="Different Laptop With Long Battery Life" src="/other.jpg">`, "Portable Laptop With Long Battery Life", "", "", false, 1},
		{"short heading does not guess image", `<h1>Phone</h1><img alt="Phone accessories and other unrelated products" src="/other.jpg">`, "Phone", "", "", false, 0},
		{"category", `<title>Dryers</title><meta property="og:image" content="/logo.png"><script type="application/ld+json">{"@type":"CollectionPage","mainEntity":{"@type":"ItemList"}}</script>`, "", "", "", true, 0},
		{"card grid", `<h1>Dryers</h1><meta property="og:image" content="/logo.png"><i data-product-id="1"></i><i data-product-id="2"></i><i data-product-id="3"></i><i data-product-id="4"></i>`, "", "", "", true, 0},
		{"ambiguous offers", `<script type="application/ld+json">{"@type":"Product","name":"Phone","offers":[{"price":10,"priceCurrency":"USD"},{"price":20,"priceCurrency":"USD"}]}</script>`, "Phone", "", "", false, 0},
		{"current page offer", `<script type="application/ld+json">{"@type":"Product","name":"Phone","offers":[{"url":"/other","price":10,"priceCurrency":"USD"},{"url":"/product","price":20,"priceCurrency":"USD"}]}</script>`, "Phone", "20.00", "USD", false, 0},
		{"malicious images", `<meta property="og:title" content="Safe"><meta property="og:image" content="http://169.254.169.254/latest"><link rel="image_src" href="data:image/svg+xml,bad">`, "Safe", "", "", false, 0},
		{"json not evaluated", `<title>Safe</title><script type="application/ld+json">{"@type":"Product","name":alert(1)}</script><gallery :product="runCode()"></gallery>`, "Safe", "", "", false, 0},
		{"currency ambiguous", `<script type="application/ld+json">{"@type":"Product","name":"Phone","offers":{"price":10,"priceCurrency":"$"}}</script>`, "Phone", "", "", false, 0},
		{"rdfa", `<div typeof="schema:Product"><span property="schema:name">RDFa gift</span><meta property="schema:price" content="20.5"><meta property="schema:priceCurrency" content="EUR"></div>`, "RDFa gift", "20.50", "EUR", false, 0},
		{"variant group", `<script type="application/ld+json">{"@type":"ProductGroup","hasVariant":[{"@type":"Product","url":"/other","name":"Other","offers":{"price":10,"priceCurrency":"EUR"}},{"@type":"Product","url":"/product","name":"Selected","offers":{"price":20,"priceCurrency":"EUR"}}]}</script>`, "Selected", "20.00", "EUR", false, 0},
		{"main offer among recommendations", `<script type="application/ld+json">[{"@type":"Product","name":"Other","offers":{"url":"/other","price":10,"priceCurrency":"EUR"}},{"@type":"Product","name":"Main","offers":{"url":"/product","price":20,"priceCurrency":"EUR"}}]</script>`, "Main", "20.00", "EUR", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := extract([]byte(tt.html), page, nil)
			if err != nil || m.Title != tt.title || m.Price != tt.price || m.Currency != tt.currency || m.NotProduct != tt.category || len(m.Images) != tt.images {
				t.Fatalf("unexpected extraction: %+v %v", m, err)
			}
		})
	}
}

func TestExactDecimal(t *testing.T) {
	for raw, want := range map[string]string{"49.950000000000003": "", "49.95000": "49.95", "0": "0.00", "1,699.50": "1699.50", "1.699,50": "1699.50", "1 699": "1699.00", "-1": "", "NaN": "", "1e99": "", "10000000000": "", "12.345": "", "0.05": "0.05"} {
		if got := decimal(raw); got != want {
			t.Errorf("%s => %s, wanted %s", raw, got, want)
		}
	}
}

func TestProductDescription(t *testing.T) {
	page, _ := url.Parse("https://shop.example.com/product")
	for _, tt := range []struct{ name, html, want string }{
		{"microdata", `<div itemscope itemtype="//schema.org/Product"><meta itemprop="name" content="Chair"><span itemprop="description">Soft <b>chair</b><script>bad()</script></span></div>`, "Soft chair"},
		{"json ld html", `<script type="application/ld+json">{"@type":"Product","name":"Chair","description":"Soft <b>chair</b>\u003cscript>bad()\u003c/script>"}</script>`, "Soft chair"},
		{"maximum escaped iframe", `<script type="application/ld+json">{"@type":"Product","name":"Vacuum","description":"&lt;iframe class=&quot;mx-richcontent-autosize&quot; src=&quot;https://plus.maximum.md/RichContentViewer/Models/332146/index.html?culture=ro&quot;&gt;&lt;/iframe&gt;"}</script>`, ""},
		{"encoded markup and frame fallback", `<script type="application/ld+json">{"@type":"Product","name":"Chair","description":"Useful &amp;lt;b&amp;gt;details&amp;lt;/b&amp;gt; &amp;lt;iframe src='https://example.org'&amp;gt;do not use&amp;lt;/iframe&amp;gt;"}</script>`, "Useful details"},
		{"microdata iframe fallback", `<div itemscope itemtype="//schema.org/Product"><div itemprop="description">Good <iframe src="https://example.org">do not use</iframe> chair <object>not visible</object></div></div>`, "Good chair"},
		{"meta", `<div itemscope itemtype="https://schema.org/Product"><meta itemprop="description" content="Warm &amp; soft"></div>`, "Warm & soft"},
		{"unrelated description", `<title>Chair</title><p>Unrelated</p><meta name="description" content="Store marketing">`, ""},
		{"category", `<script type="application/ld+json">{"@type":"CollectionPage","description":"Category"}</script>`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extract([]byte(tt.html), page, nil)
			if err != nil || got.Description != tt.want {
				t.Fatalf("description = %q, err = %v", got.Description, err)
			}
		})
	}
	long := strings.Repeat("я", 5100)
	m, err := extract([]byte(`<div itemscope itemtype="//schema.org/Product"><span itemprop="description">`+long+`</span></div>`), page, nil)
	if err != nil || len([]rune(m.Description)) != 5000 {
		t.Fatalf("description limit: %d, %v", len([]rune(m.Description)), err)
	}
}

type markupAdapter struct{}

func (markupAdapter) Match(*url.URL) bool { return true }
func (markupAdapter) Enrich(_ Document, m Metadata) Metadata {
	m.Title = "&lt;b&gt;Adapter title&lt;/b&gt;"
	m.Description = "Text &lt;iframe&gt;do not import&lt;/iframe&gt;"
	m.ShopName = "<b>Shop</b><script>ignored</script>"
	return m
}

func TestImportedTextNormalization(t *testing.T) {
	page, _ := url.Parse("https://shop.example.com/product")
	for _, tt := range []struct{ name, html, title, description, shop, price, currency string }{
		{"json ld", `<meta property="og:site_name" content="&lt;b&gt;Shop&lt;/b&gt;"><script type="application/ld+json">{"@type":"Product","name":"&lt;b&gt;Chair&lt;/b&gt;","description":"Nice &lt;i&gt;chair&lt;/i&gt;","offers":{"price":"<b>45</b>","priceCurrency":"<b>MDL</b>"}}</script>`, "Chair", "Nice chair", "Shop", "45.00", "MDL"},
		{"og title", `<meta property="og:title" content="&lt;b&gt;Chair&lt;/b&gt;"><meta property="og:site_name" content="&lt;i&gt;Shop&lt;/i&gt;">`, "Chair", "", "Shop", "", ""},
		{"literal comparisons", `<script type="application/ld+json">{"@type":"Product","name":"A & B <3","description":"Less than 5 < 10 and 10 > 5"}</script>`, "A & B <3", "Less than 5 < 10 and 10 > 5", "", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extract([]byte(tt.html), page, nil)
			if err != nil || got.Title != tt.title || got.Description != tt.description || got.ShopName != tt.shop || got.Price != tt.price || got.Currency != tt.currency {
				t.Fatalf("unexpected metadata: %+v, %v", got, err)
			}
		})
	}
	m, err := extract([]byte(`<title>Original</title>`), page, []Adapter{markupAdapter{}})
	if err != nil || m.Title != "Adapter title" || m.Description != "Text" || m.ShopName != "Shop" {
		t.Fatalf("adapter output not normalized: %+v, %v", m, err)
	}
}

type exampleAdapter struct{}

func (exampleAdapter) Match(u *url.URL) bool { return u.Hostname() == "special.example.com" }
func (exampleAdapter) Enrich(d Document, m Metadata) Metadata {
	m.Title = "Adapted"
	m.Images = []string{"http://127.0.0.1/secret", "/valid.jpg"}
	return m
}
func TestAdapterAndBounds(t *testing.T) {
	for _, host := range []string{"shop.example.com", "special.example.com"} {
		u, _ := url.Parse("https://" + host + "/product")
		m, err := extract([]byte(`<title>Common</title>`), u, []Adapter{exampleAdapter{}})
		if err != nil {
			t.Fatal(err)
		}
		if host == "special.example.com" {
			if m.Title != "Adapted" || len(m.Images) != 1 {
				t.Fatalf("adapter bypassed normalization: %+v", m)
			}
		} else if m.Title != "Common" {
			t.Fatal("adapter matched unrelated host")
		}
	}
	if decodeJSON(strings.Repeat("[", 40)+strings.Repeat("]", 40)) != nil {
		t.Fatal("unbounded JSON depth")
	}
}
