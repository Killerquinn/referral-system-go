package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func TestProcess(t *testing.T) {
	if err := Startup(); err != nil {
		t.Fatal(err)
	}
	defer Shutdown()

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
