// Package media turns an uploads row (joined into another feature's query) into what the API returns.
//
// Features never store storage keys: they keep an upload id and LEFT JOIN uploads when reading.
// Only ready files from the public bucket get a URL here — private files (paid post media) need
// a presigned GET after an access check, which is a different path.
package media

// URLBuilder is the subset of *r2.Client needed to build public URLs.
// Pass an UNTYPED nil when storage is disabled (a nil *r2.Client in the interface is != nil).
type URLBuilder interface {
	PublicURL(key string) string
}

// Row is what a query selects from the joined uploads row. All nullable: LEFT JOIN, or a file
// that isn't ready / isn't public (the JOIN condition filters those out, so they arrive as NULLs).
type Row struct {
	MediaType  *string
	StorageKey *string
	ThumbKey   *string // variants->>'thumb', images only
	Blurhash   *string
	Width      *int
	Height     *int
}

// File is the API shape of an attached file.
type File struct {
	MediaType string  `json:"media_type"` // image / video / audio
	URL       string  `json:"url"`
	ThumbURL  *string `json:"thumb_url,omitempty"`
	Blurhash  *string `json:"blurhash,omitempty"`
	Width     *int    `json:"width,omitempty"`
	Height    *int    `json:"height,omitempty"`
}

// Build returns nil when there is nothing to show (no file attached, not ready, or storage disabled).
func Build(u URLBuilder, r Row) *File {
	if u == nil || r.StorageKey == nil || r.MediaType == nil {
		return nil
	}

	f := &File{
		MediaType: *r.MediaType,
		URL:       u.PublicURL(*r.StorageKey),
		Blurhash:  r.Blurhash,
		Width:     r.Width,
		Height:    r.Height,
	}
	if r.ThumbKey != nil && *r.ThumbKey != "" {
		thumb := u.PublicURL(*r.ThumbKey)
		f.ThumbURL = &thumb
	}
	return f
}

// JoinColumns is the column list for a LEFT JOIN of uploads under alias `a` — select it in this
// order and Scan into Row.Targets(). Usage:
//
//	SELECT ..., ` + media.JoinColumns("av") + `
//	FROM users u
//	LEFT JOIN uploads av ON ` + media.JoinOn("av", "u.avatar_upload_id") + `
func JoinColumns(a string) string {
	return a + ".media_type, " + a + ".storage_key, " + a + ".variants->>'thumb', " +
		a + ".blurhash, " + a + ".width, " + a + ".height"
}

// JoinOn matches only files that can be shown publicly.
func JoinOn(a, idColumn string) string {
	return a + ".id = " + idColumn + " AND " + a + ".status = 'ready' AND " + a + ".bucket = 'public'"
}

// Targets returns Scan destinations in JoinColumns order.
func (r *Row) Targets() []any {
	return []any{&r.MediaType, &r.StorageKey, &r.ThumbKey, &r.Blurhash, &r.Width, &r.Height}
}
