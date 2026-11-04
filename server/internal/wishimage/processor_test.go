package wishimage

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestProcessReencodesResizesAndStripsPayload(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2000, 1000))
	source.Set(500, 500, color.NRGBA{R: 255, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, source); err != nil {
		t.Fatal(err)
	}
	data.WriteString("private metadata and unexpected trailing payload")
	result, err := New().Process(t.Context(), data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(result))
	if err != nil || format != "jpeg" || config.Width != 1280 || config.Height != 640 || len(result) > MaxStoredBytes {
		t.Fatalf("invalid output: %+v %s %v", config, format, err)
	}
	if bytes.Contains(result, []byte("private metadata")) {
		t.Fatal("original payload survived re-encoding")
	}
	decoded, err := jpeg.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decoded.At(0, 0).RGBA()
	if r < 65000 || g < 65000 || b < 65000 {
		t.Fatal("transparency was not composited on white")
	}
}

func TestProcessRejectsUnsafeOrOversizedFiles(t *testing.T) {
	// A valid PNG header declaring excessive pixel dimensions, without allocating
	// the corresponding raster. DecodeConfig must reject this before Decode.
	oversized := make([]byte, 33)
	copy(oversized, []byte("\x89PNG\r\n\x1a\n"))
	binary.BigEndian.PutUint32(oversized[8:12], 13)
	copy(oversized[12:16], "IHDR")
	binary.BigEndian.PutUint32(oversized[16:20], 8000)
	binary.BigEndian.PutUint32(oversized[20:24], 8000)
	oversized[24], oversized[25] = 8, 2
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	for _, test := range []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, ErrInvalid},
		{"script renamed as image", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), ErrInvalid},
		{"broken PNG", []byte("\x89PNG\r\n\x1a\n"), ErrInvalid},
		{"oversized upload", make([]byte, MaxUploadBytes+1), ErrTooLarge},
		{"pixel bomb", oversized, ErrDimensions},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New().Process(t.Context(), test.data); err != test.want {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestJPEGOrientation(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 40, 20))
	var original bytes.Buffer
	if err := jpeg.Encode(&original, source, nil); err != nil {
		t.Fatal(err)
	}
	for orientation := 1; orientation <= 8; orientation++ {
		tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, byte(orientation), 0, 0, 0, 0, 0, 0, 0}
		segment := append([]byte("Exif\x00\x00"), tiff...)
		withEXIF := append([]byte{0xff, 0xd8, 0xff, 0xe1, byte((len(segment) + 2) >> 8), byte(len(segment) + 2)}, segment...)
		withEXIF = append(withEXIF, original.Bytes()[2:]...)
		if jpegOrientation(withEXIF) != orientation {
			t.Fatalf("orientation %d not detected", orientation)
		}
		result, err := New().Process(t.Context(), withEXIF)
		if err != nil {
			t.Fatal(err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(result))
		width, height := 40, 20
		if orientation >= 5 {
			width, height = height, width
		}
		if err != nil || config.Width != width || config.Height != height || bytes.Contains(result, []byte("Exif")) {
			t.Fatalf("orientation %d: %+v %v", orientation, config, err)
		}
	}
	// Malformed EXIF offsets are ignored safely and stripped on re-encoding.
	for size := 0; size < original.Len(); size++ {
		_ = jpegOrientation(original.Bytes()[:size])
	}
}

func TestOrientPixelMappingAndCancellation(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(1 + x + y*2), A: 255})
		}
	}
	want := [][]uint8{{1, 2, 3, 4, 5, 6}, {2, 1, 4, 3, 6, 5}, {6, 5, 4, 3, 2, 1}, {5, 6, 3, 4, 1, 2}, {1, 3, 5, 2, 4, 6}, {5, 3, 1, 6, 4, 2}, {6, 4, 2, 5, 3, 1}, {2, 4, 6, 1, 3, 5}}
	for orientation := 1; orientation <= 8; orientation++ {
		result := orient(source, orientation)
		index := 0
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				red, _, _, _ := result.At(x, y).RGBA()
				if uint8(red>>8) != want[orientation-1][index] {
					t.Fatalf("orientation %d pixel %d", orientation, index)
				}
				index++
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	processor := New()
	processor.slots <- struct{}{}
	processor.slots <- struct{}{}
	if _, err := processor.Process(ctx, []byte("image")); err != context.Canceled {
		t.Fatalf("cancelled decoder wait: %v", err)
	}
}
