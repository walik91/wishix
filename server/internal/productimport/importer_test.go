package productimport

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

func TestPreviewFallsBackAndNormalizesImage(t *testing.T) {
	var raw bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	_ = png.Encode(&raw, img)
	var paths []string
	f, _ := testFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/product":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Gift","image":["/broken.jpg","/broken.jpg","/fake.png"],"offers":{"price":"49.95","priceCurrency":"MDL"}}</script><gallery :imgs='"/valid.png"'></gallery>`))
		case "/fake.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(`<svg><script>bad</script></svg>`))
		case "/valid.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(raw.Bytes())
		default:
			w.WriteHeader(404)
		}
	})
	i := New()
	i.fetch = f
	result, err := i.Preview(context.Background(), "http://shop.example.com/product", "ru")
	if err != nil || result.Status != "ok" || result.Image == nil {
		t.Fatalf("preview: %+v %v", result, err)
	}
	data, err := base64.StdEncoding.DecodeString(result.Image.Data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal("not a sanitized JPEG", err)
	}
	if strings.Join(paths, ",") != "/product,/broken.jpg,/fake.png,/valid.png" {
		t.Fatalf("unexpected image attempts %v", paths)
	}
}

func TestImageFailureKeepsMetadataAndCategorySkipsImages(t *testing.T) {
	for _, category := range []bool{false, true} {
		calls := 0
		f, _ := testFetcher(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "text/html")
			if r.URL.Path != "/product" {
				w.WriteHeader(404)
				return
			}
			raw := `<title>Gift</title><meta property="og:image" content="/broken.jpg">`
			if category {
				raw += `<script type="application/ld+json">{"@type":"CollectionPage"}</script>`
			}
			_, _ = w.Write([]byte(raw))
		})
		i := New()
		i.fetch = f
		r, err := i.Preview(context.Background(), "http://shop.example.com/product", "en")
		if err != nil {
			t.Fatal(err)
		}
		if category {
			if r.Status != "not_product" || r.Title != "" || calls != 1 {
				t.Fatalf("category imported data %+v", r)
			}
		} else if r.Status != "partial" || r.Title != "Gift" || r.Image != nil {
			t.Fatalf("partial result lost %+v", r)
		}
	}
}

func TestConcurrencyBound(t *testing.T) {
	i := New()
	i.slots <- struct{}{}
	i.slots <- struct{}{}
	if _, err := i.Preview(context.Background(), "https://shop.example.com/product", "en"); err != ErrBusy {
		t.Fatal("busy importer accepted more work")
	}
}
