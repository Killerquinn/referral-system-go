// Package r2 is a thin wrapper over Cloudflare R2 (S3-compatible API).
//
// Upload flow: backend issues a presigned PUT (PresignPut) -> client uploads the file
// directly to R2 -> client tells backend "done" -> backend verifies with Head
// (size / content type) before marking the upload ready.
package r2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/killerquinn/referral-system-go/internal/config"
)

var (
	ErrNotConfigured  = errors.New("r2: storage is not configured")
	ErrObjectNotFound = errors.New("r2: object not found")
)

type Client struct {
	s3            *s3.Client
	presign       *s3.PresignClient
	publicBucket  string
	privateBucket string
	publicBaseURL string
	ttl           time.Duration
}

// PresignedUpload is what the frontend needs to upload a file directly to R2:
// a PUT to URL with Headers set (the browser adds Content-Length itself).
type PresignedUpload struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}

func New(cfg config.R2Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" ||
		cfg.PublicBucket == "" || cfg.PrivateBucket == "" || cfg.PublicBaseURL == "" {
		return nil, ErrNotConfigured
	}

	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		// Newer SDK versions add CRC checksums to every request by default. In presigned URLs
		// they become signed query params the browser can't satisfy, so only send them when required.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})

	return &Client{
		s3:            client,
		presign:       s3.NewPresignClient(client),
		publicBucket:  cfg.PublicBucket,
		privateBucket: cfg.PrivateBucket,
		publicBaseURL: cfg.PublicBaseURL,
		ttl:           cfg.PresignTTL,
	}, nil
}

func (c *Client) PublicBucket() string  { return c.publicBucket }
func (c *Client) PrivateBucket() string { return c.privateBucket }

// CheckBuckets verifies credentials and that both buckets exist. Call on startup.
func (c *Client) CheckBuckets(ctx context.Context) error {
	for _, b := range []string{c.publicBucket, c.privateBucket} {
		if _, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(b)}); err != nil {
			return fmt.Errorf("r2: bucket %q: %w", b, err)
		}
	}
	return nil
}

// PresignPut returns a URL for uploading exactly `size` bytes of `contentType` to bucket/key.
// Content-Type and Content-Length are part of the signature, so the client can't upload
// a bigger file or a different type than the backend agreed to.
func (c *Client) PresignPut(ctx context.Context, bucket, key, contentType string, size int64) (*PresignedUpload, error) {
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(c.ttl))
	if err != nil {
		return nil, fmt.Errorf("r2: presign put: %w", err)
	}

	headers := make(map[string]string, len(req.SignedHeader))
	for name, values := range req.SignedHeader {
		// Browsers forbid setting these manually; they're sent automatically.
		switch http.CanonicalHeaderKey(name) {
		case "Host", "Content-Length":
			continue
		}
		headers[name] = strings.Join(values, ",")
	}

	return &PresignedUpload{
		URL:       req.URL,
		Method:    req.Method,
		Headers:   headers,
		ExpiresAt: time.Now().Add(c.ttl),
	}, nil
}

// PresignGet returns a short-lived download URL (for private bucket objects).
func (c *Client) PresignGet(ctx context.Context, bucket, key string) (string, time.Time, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(c.ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("r2: presign get: %w", err)
	}
	return req.URL, time.Now().Add(c.ttl), nil
}

// PublicURL builds a permanent URL for an object in the public bucket.
func (c *Client) PublicURL(key string) string {
	segments := strings.Split(key, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return c.publicBaseURL + "/" + strings.Join(segments, "/")
}

// Head returns object metadata; ErrObjectNotFound if the object doesn't exist
// (e.g. the client never finished the upload).
func (c *Client) Head(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("r2: head: %w", err)
	}
	return &ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ETag:        aws.ToString(out.ETag),
	}, nil
}

// Get opens the object for reading (for processing jobs). Caller must close the body.
func (c *Client) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil, ErrObjectNotFound
		}
		return nil, nil, fmt.Errorf("r2: get: %w", err)
	}
	return out.Body, &ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ETag:        aws.ToString(out.ETag),
	}, nil
}

// Put uploads from the server side (e.g. processed thumbnails).
func (c *Client) Put(ctx context.Context, bucket, key, contentType string, body io.Reader, size int64) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("r2: put: %w", err)
	}
	return nil
}

// Delete removes an object. Deleting a missing object is not an error (S3 semantics).
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	if _, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("r2: delete: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	var nf *types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return true
	}
	// HeadObject returns a bare 404 without a typed error body.
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey")
}
