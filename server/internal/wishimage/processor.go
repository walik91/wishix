package wishimage

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const MaxUploadBytes = 5 * 1024 * 1024
const MaxStoredBytes = 512 * 1024
const MaxPixels = 20_000_000

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrInvalid    Error = "wish_image_invalid"
	ErrTooLarge   Error = "wish_image_too_large"
	ErrDimensions Error = "wish_image_dimensions"
)

// Limit decoded pixels AND concurrent decoders: compressed file size alone
// does not bound the memory needed by a raster image.
type Processor struct{ slots chan struct{} }

func New() *Processor { return &Processor{slots: make(chan struct{}, 2)} }

func (p *Processor) Process(ctx context.Context, raw []byte) ([]byte, error) {
	if len(raw) > MaxUploadBytes {
		return nil, ErrTooLarge
	}
	if len(raw) == 0 {
		return nil, ErrInvalid
	}
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, ErrInvalid
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || config.Width > MaxPixels/config.Height {
		return nil, ErrDimensions
	}
	source, decodedFormat, err := image.Decode(bytes.NewReader(raw))
	if err != nil || decodedFormat != format || source.Bounds().Dx() != config.Width || source.Bounds().Dy() != config.Height {
		return nil, ErrInvalid
	}
	orientation := 1
	if format == "jpeg" {
		orientation = jpegOrientation(raw)
	}
	for _, side := range []int{1280, 1024, 768, 512} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		width, height := config.Width, config.Height
		if largest := max(width, height); largest > side {
			width, height = max(1, width*side/largest), max(1, height*side/largest)
		}
		canvas := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), source, source.Bounds(), draw.Over, nil)
		var output bytes.Buffer
		if err := jpeg.Encode(&output, orient(canvas, orientation), &jpeg.Options{Quality: 82}); err != nil {
			return nil, ErrInvalid
		}
		if output.Len() <= MaxStoredBytes {
			return output.Bytes(), nil
		}
	}
	return nil, ErrTooLarge
}

// Read only the bounded TIFF orientation field; original EXIF and all other
// metadata are discarded by JPEG re-encoding, including any trailing payload.
func jpegOrientation(raw []byte) int {
	if len(raw) < 4 || raw[0] != 0xff || raw[1] != 0xd8 {
		return 1
	}
	for cursor := 2; cursor+4 <= len(raw) && raw[cursor] == 0xff; {
		marker := raw[cursor+1]
		cursor += 2
		if marker == 0xda || marker == 0xd9 {
			break
		}
		if marker == 0xff {
			cursor--
			continue
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		size := int(binary.BigEndian.Uint16(raw[cursor : cursor+2]))
		if size < 2 || size > len(raw)-cursor {
			return 1
		}
		segment := raw[cursor+2 : cursor+size]
		cursor += size
		if marker != 0xe1 || !bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			continue
		}
		tiff := segment[6:]
		if len(tiff) < 8 {
			return 1
		}
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 1
		}
		if order.Uint16(tiff[2:4]) != 42 {
			return 1
		}
		offset := uint64(order.Uint32(tiff[4:8]))
		if offset+2 > uint64(len(tiff)) {
			return 1
		}
		count := uint64(order.Uint16(tiff[offset : offset+2]))
		if offset+2+count*12 > uint64(len(tiff)) {
			return 1
		}
		for index := uint64(0); index < count; index++ {
			entry := tiff[offset+2+index*12 : offset+2+(index+1)*12]
			if order.Uint16(entry[:2]) == 0x112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
				if orientation := int(order.Uint16(entry[8:10])); orientation >= 1 && orientation <= 8 {
					return orientation
				}
			}
		}
		return 1
	}
	return 1
}

func orient(source *image.RGBA, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return source
	}
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	bounds := image.Rect(0, 0, width, height)
	if orientation >= 5 {
		bounds = image.Rect(0, 0, height, width)
	}
	result := image.NewRGBA(bounds)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = width - 1 - x
			case 3:
				dx, dy = width-1-x, height-1-y
			case 4:
				dy = height - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = height-1-y, x
			case 7:
				dx, dy = height-1-y, width-1-x
			case 8:
				dx, dy = y, width-1-x
			}
			result.SetRGBA(dx, dy, source.RGBAAt(x, y))
		}
	}
	return result
}
