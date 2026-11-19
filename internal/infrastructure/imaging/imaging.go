// Package imaging turns an uploaded image into web-ready WebP variants with libvips (govips).
//
// SECURITY: libvips can load many formats (PDF, SVG, ImageMagick, ...), and govips picks the
// loader from the bytes. Only call Process for content already sniffed as one of the allowed
// image types (see Sniff) — never on arbitrary input.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/buckket/go-blurhash"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/gabriel-vasile/mimetype"
)

// ErrInvalidImage means the file is broken or not acceptable (too many pixels, can't decode).
// Every error from Process wraps it: processing is pure CPU work on bytes already in memory,
// so a retry would fail the same way. libvips decodes lazily — a truncated file often loads
// fine and only fails later while encoding — so encode errors count as "invalid" too.
var ErrInvalidImage = errors.New("imaging: invalid image")

const (
	// decompression-bomb guard: a tiny file can claim gigapixel dimensions
	maxPixels         = 40_000_000 // ~ 8000x5000
	quality           = 82
	sniffLen          = 3072
	maxFrames         = 300
	maxAnimatedPixels = 100_000_000
)

// Variant sizes: longest side in px. Images are only ever shrunk, never upscaled.
var sizes = []struct {
	name string
	max  int
}{
	{"thumb", 320},
	{"medium", 1080},
	{"full", 2048},
}

type Variant struct {
	Name   string // thumb / medium / full
	Data   []byte // WebP
	Width  int
	Height int
}

type Result struct {
	Variants []Variant
	Width    int // of the original, after EXIF rotation
	Height   int
	Blurhash string
}

// Startup must be called once before Process (in main), Shutdown on exit.
func Startup() error {
	vips.LoggingSettings(nil, vips.LogLevelWarning)
	return vips.Startup(&vips.Config{
		ConcurrencyLevel: 1,        // the worker pool gives the parallelism
		MaxCacheMem:      50 << 20, // we never process the same image twice
		MaxCacheSize:     100,
	})
}

func Shutdown() { vips.Shutdown() }

// Sniff detects the real content type from the first bytes (the client's declared type is not trusted).
// Returns e.g. "image/jpeg", "video/mp4", "audio/mpeg"; "application/octet-stream" if unknown.
// The returned reader replays the sniffed bytes, so the caller can keep streaming from it.
func Sniff(r io.Reader) (string, io.Reader, error) {
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", nil, fmt.Errorf("imaging: sniff: %w", err)
	}
	head = head[:n]
	return mimetype.Detect(head).String(), io.MultiReader(bytes.NewReader(head), r), nil
}

// load decodes data to pages where (-1 = all frames, 1 = first frame only)
// also load used to delete all metadata from []data and provides EXIF rotation
// The caller must Close() returned type *vips.ImageRef to avoid data leak. All load errors wrap ErrInvalidImage
func load(data []byte, pages int) (*vips.ImageRef, error) {
	params := vips.NewImportParams()
	params.FailOnError.Set(true)
	params.NumPages.Set(pages)

	img, err := vips.LoadImageFromBuffer(data, params)
	if err != nil {
		return nil, invalid("decode", err)
	}

	if err := img.AutoRotate(); err != nil {
		img.Close()
		return nil, invalid("rotate", err)
	}
	if err := img.RemoveMetadata(); err != nil {
		img.Close()
		return nil, invalid("strip metadata", err)
	}

	return img, nil
}

// Process decodes an image, applies EXIF rotation, strips all metadata (GPS!) and encodes
// WebP variants plus a blurhash. Animations stay animated in medium and full; thumb and
// blurhash are made from the first frame.
// Every error wraps [ErrInvalidImage]: the input is bad, a retry won't help.
// Startup must be called before.
func Process(data []byte) (*Result, error) {

	img, err := load(data, -1)
	if err != nil {
		return nil, err
	}

	defer img.Close()
	frames := img.Pages()
	frameH := img.PageHeight()

	if int64(img.Width())*int64(frameH) > maxPixels {
		return nil, fmt.Errorf("%w: %dx%d is too large", ErrInvalidImage, img.Width(), frameH)
	}

	if int64(frames) > maxFrames {
		return nil, fmt.Errorf("%w: %d", ErrInvalidImage, frames)
	}

	if int64(img.Width())*int64(img.Height()) > maxAnimatedPixels {
		return nil, fmt.Errorf("%w: %dx%d is too large", ErrInvalidImage, img.Width(), img.Height())
	}

	first := img
	if frames > 1 {
		first, err = load(data, 1)
		if err != nil {
			return nil, err
		}
		defer first.Close()
	}

	res := &Result{Width: img.Width(), Height: img.PageHeight()}

	export := vips.NewWebpExportParams()
	export.Quality = quality
	export.StripMetadata = true

	for _, s := range sizes {
		src := img
		if s.name == "thumb" {
			src = first
		}

		v, err := resized(src, s.max)
		if err != nil {
			return nil, invalid("resize "+s.name, err)
		}
		buf, _, err := v.ExportWebp(export)
		w, h := v.Width(), v.PageHeight()
		v.Close()
		if err != nil {
			return nil, invalid("encode "+s.name, err)
		}
		res.Variants = append(res.Variants, Variant{Name: s.name, Data: buf, Width: w, Height: h})
	}

	// blurhash from a tiny copy: the algorithm is slow on big images and doesn't need detail
	small, err := resized(first, 32)
	if err != nil {
		return nil, invalid("blurhash resize", err)
	}
	defer small.Close()
	goImg, err := small.ToImage(vips.NewDefaultExportParams())
	if err != nil {
		return nil, invalid("blurhash decode", err)
	}
	if res.Blurhash, err = blurhash.Encode(4, 3, goImg); err != nil {
		return nil, invalid("blurhash", err)
	}

	return res, nil
}

// invalid wraps err as ErrInvalidImage. govips appends a Go stack trace to its error text — keep the first line only.
func invalid(stage string, err error) error {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	return fmt.Errorf("%w: %s: %s", ErrInvalidImage, stage, strings.TrimSpace(msg))
}

// resized returns a copy that fits into max x max (keeps aspect ratio, never upscales).
func resized(img *vips.ImageRef, max int) (*vips.ImageRef, error) {
	c, err := img.Copy()
	if err != nil {
		return nil, err
	}
	if err := c.ThumbnailWithSize(max, max, vips.InterestingNone, vips.SizeDown); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
