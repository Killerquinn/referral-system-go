package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/killerquinn/referral-system-go/internal/features/upload/dto"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/imaging"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/storage/r2"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	if err := imaging.Startup(); err != nil {
		panic(err)
	}
	code := m.Run()
	imaging.Shutdown()
	os.Exit(code)
}

// ===== fakes =====

type fakeStore struct {
	uploadStorage // unused methods panic (nil interface) — fine for these tests
	row           dto.UploadRow
	readyErr      error
	ready         *dto.MarkReadyParams
	failedStatus  string
	failedReason  string
}

func (f *fakeStore) GetUpload(context.Context, string) (dto.UploadRow, error) { return f.row, nil }

func (f *fakeStore) MarkReady(_ context.Context, p dto.MarkReadyParams) error {
	if f.readyErr != nil {
		return f.readyErr
	}
	f.ready = &p
	return nil
}

func (f *fakeStore) MarkFailed(_ context.Context, _, status, reason string) error {
	f.failedStatus, f.failedReason = status, reason
	return nil
}

type fakeObjects struct {
	files map[string][]byte // "bucket/key" -> data
}

func (o *fakeObjects) PresignPut(context.Context, string, string, string, int64) (*r2.PresignedUpload, error) {
	return nil, errors.New("not used")
}
func (o *fakeObjects) Head(context.Context, string, string) (*r2.ObjectInfo, error) {
	return nil, errors.New("not used")
}
func (o *fakeObjects) Get(_ context.Context, bucket, key string) (io.ReadCloser, *r2.ObjectInfo, error) {
	b, ok := o.files[bucket+"/"+key]
	if !ok {
		return nil, nil, r2.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), &r2.ObjectInfo{Size: int64(len(b))}, nil
}
func (o *fakeObjects) Put(_ context.Context, bucket, key, _ string, body io.Reader, _ int64) error {
	b, _ := io.ReadAll(body)
	o.files[bucket+"/"+key] = b
	return nil
}
func (o *fakeObjects) Delete(_ context.Context, bucket, key string) error {
	delete(o.files, bucket+"/"+key)
	return nil
}
func (o *fakeObjects) PublicURL(key string) string { return "https://cdn/" + key }
func (o *fakeObjects) PublicBucket() string        { return "pub" }
func (o *fakeObjects) PrivateBucket() string       { return "priv" }

func pngBytes(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func setup(purpose, mediaType string, incoming []byte) (*UploadService, *fakeStore, *fakeObjects) {
	size := int64(len(incoming))
	store := &fakeStore{row: dto.UploadRow{
		ID: "u1", OwnerID: "owner", Purpose: purpose, MediaType: mediaType,
		Status: dto.StatusProcessing, IncomingKey: "incoming/u1",
		DeclaredContentType: "image/png", DeclaredSizeBytes: size, SizeBytes: &size,
		CreatedAt: time.Now(),
	}}
	objs := &fakeObjects{files: map[string][]byte{"priv/incoming/u1": incoming}}
	return New(zap.NewNop(), store, objs, nil), store, objs
}

func keys(o *fakeObjects) string {
	var ks []string
	for k := range o.files {
		ks = append(ks, k)
	}
	return strings.Join(ks, ",")
}

// ===== tests =====

func TestProcessUpload_ImageReady(t *testing.T) {
	us, store, objs := setup(dto.PurposeAvatar, dto.MediaImage, pngBytes(t, 600, 400))

	if err := us.ProcessUpload(context.Background(), "u1", 1, 5); err != nil {
		t.Fatal(err)
	}
	if store.ready == nil {
		t.Fatalf("not marked ready (failed: %q %q)", store.failedStatus, store.failedReason)
	}
	p := store.ready
	if p.Bucket != dto.BucketPublic || p.StorageKey != "media/u1/full.webp" || p.Variants.Thumb != "media/u1/thumb.webp" {
		t.Fatalf("bad params: %+v", p)
	}
	if *p.Width != 600 || *p.Height != 400 || *p.Blurhash == "" || p.MimeType != "image/png" || p.SizeBytes <= 0 {
		t.Fatalf("bad meta: %+v", p)
	}
	if _, ok := objs.files["priv/incoming/u1"]; ok {
		t.Fatal("incoming file must be deleted")
	}
	for _, k := range []string{"pub/media/u1/thumb.webp", "pub/media/u1/medium.webp", "pub/media/u1/full.webp"} {
		if _, ok := objs.files[k]; !ok {
			t.Fatalf("missing %s, have %s", k, keys(objs))
		}
	}
}

func TestProcessUpload_PostMediaGoesPrivate(t *testing.T) {
	us, store, objs := setup(dto.PurposePostMedia, dto.MediaImage, pngBytes(t, 50, 50))
	if err := us.ProcessUpload(context.Background(), "u1", 1, 5); err != nil {
		t.Fatal(err)
	}
	if store.ready == nil || store.ready.Bucket != dto.BucketPrivate {
		t.Fatalf("post media must be private: %+v", store.ready)
	}
	if _, ok := objs.files["priv/media/u1/full.webp"]; !ok {
		t.Fatalf("expected file in private bucket, have %s", keys(objs))
	}
}

func TestProcessUpload_FakeImageFails(t *testing.T) {
	// "declared" as an image, actually a script
	us, store, objs := setup(dto.PurposeAvatar, dto.MediaImage, []byte("#!/bin/sh\nrm -rf /\n"))

	if err := us.ProcessUpload(context.Background(), "u1", 1, 5); err != nil {
		t.Fatalf("a bad file must not be retried, got %v", err)
	}
	if store.ready != nil || store.failedStatus != dto.StatusFailed {
		t.Fatalf("want failed, got ready=%v status=%q", store.ready, store.failedStatus)
	}
	if len(objs.files) != 1 { // only the incoming file stays (cleanup removes it with the row)
		t.Fatalf("nothing should be written, have %s", keys(objs))
	}
}

func TestProcessUpload_BrokenImageFails(t *testing.T) {
	b := pngBytes(t, 100, 100)
	us, store, _ := setup(dto.PurposeAvatar, dto.MediaImage, b[:len(b)/3]) // truncated PNG

	if err := us.ProcessUpload(context.Background(), "u1", 1, 5); err != nil {
		t.Fatalf("a broken image must not be retried, got %v", err)
	}
	if store.failedStatus != dto.StatusFailed {
		t.Fatalf("want failed, got %q", store.failedStatus)
	}
}

func TestProcessUpload_QuotaExceededRejectsAndCleansUp(t *testing.T) {
	us, store, objs := setup(dto.PurposeAvatar, dto.MediaImage, pngBytes(t, 100, 100))
	store.readyErr = sharederrors.ErrStorageQuotaExceeded

	if err := us.ProcessUpload(context.Background(), "u1", 1, 5); err != nil {
		t.Fatal(err)
	}
	if store.failedStatus != dto.StatusRejected {
		t.Fatalf("want rejected, got %q", store.failedStatus)
	}
	if len(objs.files) != 1 {
		t.Fatalf("processed files must be removed, have %s", keys(objs))
	}
}

func TestProcessUpload_TemporaryErrorRetriesThenFails(t *testing.T) {
	us, store, _ := setup(dto.PurposeAvatar, dto.MediaImage, pngBytes(t, 100, 100))
	store.readyErr = errors.New("db is down")

	if err := us.ProcessUpload(context.Background(), "u1", 1, 5); err == nil || store.failedStatus != "" {
		t.Fatalf("attempt 1: want error (retry) and no mark, got %v %q", err, store.failedStatus)
	}
	if err := us.ProcessUpload(context.Background(), "u1", 5, 5); err == nil || store.failedStatus != dto.StatusFailed {
		t.Fatalf("last attempt: want error and failed, got %v %q", err, store.failedStatus)
	}
}
