package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"os"
	"strings"
	"testing"
)

// TestMain starts libvips once for the whole package: govips can't be restarted after Shutdown,
// so per-test Startup/Shutdown would break every test after the first one.
func TestMain(m *testing.M) {
	if err := Startup(); err != nil {
		panic(err)
	}
	code := m.Run()
	Shutdown()
	os.Exit(code)
}

func TestProcess(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3000, 1500))
	for x := 0; x < 3000; x++ {
		for y := 0; y < 1500; y++ {
			src.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}

	ct, _, err := Sniff(bytes.NewReader(buf.Bytes()))
	if err != nil || ct != "image/jpeg" {
		t.Fatalf("sniff: %q %v", ct, err)
	}

	res, err := Process(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 3000 || res.Height != 1500 || res.Blurhash == "" || len(res.Variants) != 3 {
		t.Fatalf("unexpected result: %+v", res)
	}
	want := map[string][2]int{"thumb": {320, 160}, "medium": {1080, 540}, "full": {2048, 1024}}
	for _, v := range res.Variants {
		if w := want[v.Name]; v.Width != w[0] || v.Height != w[1] {
			t.Errorf("%s: got %dx%d want %v", v.Name, v.Width, v.Height, w)
		}
		if !bytes.HasPrefix(v.Data, []byte("RIFF")) {
			t.Errorf("%s is not webp", v.Name)
		}
	}

	if _, err := Process([]byte("definitely not an image")); err == nil || !strings.Contains(err.Error(), "invalid image") {
		t.Fatalf("garbage must be ErrInvalidImage, got %v", err)
	}
}

// makeGIF builds an animated GIF: every frame is filled with its own solid color.
func makeGIF(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		p := image.NewPaletted(image.Rect(0, 0, w, h), palette.Plan9)
		c := color.RGBA{uint8(i * 40), uint8(255 - i*40), 120, 255}
		draw.Draw(p, p.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10) // 1/100 s units -> 100 ms
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcessAnimated(t *testing.T) {
	res, err := Process(makeGIF(t, 600, 400, 5))
	if err != nil {
		t.Fatal(err)
	}

	// frame size, not the "toilet roll" (600x2000)
	if res.Width != 600 || res.Height != 400 {
		t.Fatalf("result: got %dx%d want 600x400", res.Width, res.Height)
	}
	if res.Blurhash == "" {
		t.Error("empty blurhash")
	}

	want := map[string]struct {
		w, h     int
		animated bool
	}{
		"thumb":  {320, 213, false}, // still: first frame only
		"medium": {600, 400, true},  // SizeDown: never upscaled
		"full":   {600, 400, true},
	}
	if len(res.Variants) != len(want) {
		t.Fatalf("got %d variants want %d", len(res.Variants), len(want))
	}
	for _, v := range res.Variants {
		wv := want[v.Name]
		if v.Width != wv.w || v.Height != wv.h {
			t.Errorf("%s: got %dx%d want %dx%d", v.Name, v.Width, v.Height, wv.w, wv.h)
		}
		// an animated WebP has an ANIM chunk; a still one doesn't
		if got := bytes.Contains(v.Data, []byte("ANIM")); got != wv.animated {
			t.Errorf("%s: animated=%v want %v", v.Name, got, wv.animated)
		}
	}
}

func TestProcessTooManyFrames(t *testing.T) {
	_, err := Process(makeGIF(t, 2, 2, maxFrames+1))
	// must be ErrInvalidImage: anything else is treated as temporary and retried by the queue
	if !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("want ErrInvalidImage, got %v", err)
	}
}
