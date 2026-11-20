package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/page/dto"
	"github.com/killerquinn/referral-system-go/internal/shared/media"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type pageStorage interface {
	// unique violation on user_id -> sharederrors.ErrPageAlreadyExists, on slug -> sharederrors.ErrSlugTaken
	CreatePage(ctx context.Context, userID, slug string, displayName *string, settings dto.PageSettings) (dto.PageItem, error)
	GetPageByUserID(ctx context.Context, userID string) (dto.PageItem, error)
	GetPageBySlug(ctx context.Context, slug string) (dto.PageWithAuthor, error)
	UpdatePage(ctx context.Context, userID string, displayName, bio *string, settings *dto.PageSettings,
		isPublished *bool, backgroundUploadID, musicUploadID *string) (dto.PageItem, error)
	ChangeSlug(ctx context.Context, userID, newSlug string) error
	IsSlugTaken(ctx context.Context, slug string) (bool, error)
	IsUploadAttachable(ctx context.Context, uploadID, ownerID, purpose string) (bool, error)

	// block data (read-only, the page module doesn't call other features' services)
	CountReferrals(ctx context.Context, authorID string) (int, error)
	GetRecentPostPreviews(ctx context.Context, authorID string, limit int) ([]dto.PostPreview, error)
	// sharederrors.ErrUserNotFound -> the viewer was deleted, treat as anonymous
	GetViewerContext(ctx context.Context, viewerID, authorID string) (dto.ViewerContext, error)

	// views
	GetPageForView(ctx context.Context, slug string) (dto.PageForView, error)
	RecordView(ctx context.Context, pageUserID string, day time.Time, visitorHash []byte) (bool, error)
	GetViewStats(ctx context.Context, pageUserID string, today time.Time) (dto.PageViews, error)
	DeleteViewVisitorsBefore(ctx context.Context, day time.Time) (int, error)
}

type PageService struct {
	log          *zap.Logger
	baseurl      string
	slugCooldown time.Duration
	storage      pageStorage
	urls         media.URLBuilder // nil when R2 is disabled -> pages render without files
	uploads      uploadDeleter
	viewSecret   []byte // keys the visitor hashes, so they can't be recomputed from a DB dump
}

// uploadDeleter removes an upload completely (row, files in R2, quota). *uploadservice.UploadService.
type uploadDeleter interface {
	DeleteUpload(ctx context.Context, userID, uploadID string) error
}

func New(log *zap.Logger, baseurl string, slugCooldown time.Duration, storage pageStorage, urls media.URLBuilder, uploads uploadDeleter, viewSecret []byte) *PageService {
	return &PageService{
		log:          log,
		baseurl:      strings.TrimRight(baseurl, "/"),
		slugCooldown: slugCooldown,
		storage:      storage,
		urls:         urls,
		uploads:      uploads,
		viewSecret:   viewSecret,
	}
}

// replaced reports whether an attachment change (nil = unchanged, "" = detach) drops the previous upload.
func replaced(previous, next *string) bool {
	return next != nil && previous != nil && *previous != *next
}

// ===== allowed values for settings (single source of truth, the frontend mirrors them in src/api/pages.ts) =====

var (
	allowedBackgroundTypes = map[string]struct{}{"color": {}, "gradient": {}, "media": {}}
	allowedCardStyles      = map[string]struct{}{"glass": {}, "solid": {}, "transparent": {}}
	allowedFonts           = map[string]struct{}{"inter": {}, "jetbrains_mono": {}, "space_grotesk": {}, "playfair": {}}
	allowedLayouts         = map[string]struct{}{"default": {}, "classic": {}, "minimal": {}}
	allowedLinkStyles      = map[string]struct{}{"buttons": {}, "connections": {}, "icons": {}}
	allowedLinkTypes       = map[string]struct{}{"telegram": {}, "discord": {}, "youtube": {}, "twitch": {}, "tiktok": {}, "instagram": {}, "github": {}, "website": {}}
	// later: "tiers" (after the tier feature), "giveaway" (after giveaways get an API)
	allowedBlocks = map[string]struct{}{"referral_button": {}, "referral_counter": {}, "view_counter": {}, "recent_posts": {}}

	// slugs that clash with frontend/backend routes or look official
	reservedSlugs = map[string]struct{}{
		"api": {}, "auth": {}, "login": {}, "logout": {}, "register": {}, "signup": {},
		"user": {}, "users": {}, "me": {}, "admin": {}, "settings": {}, "social": {},
		"posts": {}, "pages": {}, "page": {}, "slug-available": {}, "static": {}, "assets": {},
		"help": {}, "support": {}, "about": {}, "terms": {}, "privacy": {}, "pricing": {},
		"dashboard": {}, "referrals": {}, "p": {}, "media": {}, "uploads": {},
	}

	// effect -> its group; each group has its own limit (effectGroupLimits)
	effectGroups = map[string]string{
		// canvas over the background
		"snow": effectGroupBackground, "particles": effectGroupBackground, "rain": effectGroupBackground,
		"stars": effectGroupBackground, "fireflies": effectGroupBackground, "sakura": effectGroupBackground,
		"smoke": effectGroupBackground,
		// how the name is drawn — they all fight over color/text-shadow, so only one
		"glow_name": effectGroupName, "shimmer_name": effectGroupName, "neon_name": effectGroupName,
		"glitch_name": effectGroupName, "smolder_name": effectGroupName,
		// independent extras
		"typewriter_bio": effectGroupExtra, "tilt_card": effectGroupExtra, "cursor_wind": effectGroupExtra,
	}
	effectGroupLimits = map[string]int{
		effectGroupBackground: 3, // was the overall limit before groups, so old pages stay valid
		effectGroupName:       1,
		effectGroupExtra:      3,
	}

	slugRe = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)
	hexRe  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

const (
	effectGroupBackground = "background"
	effectGroupName       = "name"
	effectGroupExtra      = "extra"

	maxLinks             = 10
	maxLinkURLLength     = 300
	recentPostsOnPage    = 5
	maxDisplayNameLength = 64
	maxBioLength         = 500

	purposeBackground = "page_background"
	purposeMusic      = "page_music"
)

// defaultSettings is what a freshly created page gets.
func defaultSettings() dto.PageSettings {
	return dto.PageSettings{
		Background: dto.Background{
			Type:     "gradient",
			Gradient: &dto.Gradient{From: "#0f172a", To: "#4c1d95", Angle: 135},
		},
		Effects: []string{},
		Theme:   dto.Theme{AccentColor: "#a78bfa", CardStyle: "glass", Font: "inter", Layout: "default", LinkStyle: "buttons"},
		Links:   []dto.Link{},
		Blocks: []dto.Block{
			{Type: "referral_button", Enabled: true},
			{Type: "referral_counter", Enabled: true},
			{Type: "view_counter", Enabled: true},
			{Type: "recent_posts", Enabled: false},
		},
	}
}

func normalizeSlug(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// validateSlug: ^[a-z0-9_-]{3,32}$ and not in reservedSlugs.
func validateSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return sharederrors.ErrInvalidSlug
	}
	if _, ok := reservedSlugs[slug]; ok {
		return sharederrors.ErrSlugReserved
	}
	return nil
}

func invalidSettings(format string, args ...any) error {
	return fmt.Errorf("%w: %s", sharederrors.ErrInvalidPageSettings, fmt.Sprintf(format, args...))
}

// validateSettings checks every value against the allowed sets / hex format / ranges / https-only URLs,
// limits and duplicates. Nil slices become empty ones so the stored JSON is always complete.
func validateSettings(s *dto.PageSettings) error {
	bg := s.Background
	if _, ok := allowedBackgroundTypes[bg.Type]; !ok {
		return invalidSettings("unknown background type %q", bg.Type)
	}
	switch bg.Type {
	case "color":
		if bg.Color == nil || !hexRe.MatchString(*bg.Color) {
			return invalidSettings("background color must be #rrggbb")
		}
		s.Background.Gradient = nil
	case "gradient":
		g := bg.Gradient
		if g == nil || !hexRe.MatchString(g.From) || !hexRe.MatchString(g.To) {
			return invalidSettings("gradient colors must be #rrggbb")
		}
		if g.Angle < 0 || g.Angle > 360 {
			return invalidSettings("gradient angle must be 0-360")
		}
		s.Background.Color = nil
	case "media":
		s.Background.Color, s.Background.Gradient = nil, nil
	}

	if s.Effects == nil {
		s.Effects = []string{}
	}
	seen := map[string]struct{}{}
	perGroup := map[string]int{}
	for _, e := range s.Effects {
		group, ok := effectGroups[e]
		if !ok {
			return invalidSettings("unknown effect %q", e)
		}
		if _, dup := seen[e]; dup {
			return invalidSettings("duplicate effect %q", e)
		}
		seen[e] = struct{}{}
		perGroup[group]++
		if perGroup[group] > effectGroupLimits[group] {
			return invalidSettings("at most %d %s effects", effectGroupLimits[group], group)
		}
	}

	t := s.Theme
	if !hexRe.MatchString(t.AccentColor) {
		return invalidSettings("accent color must be #rrggbb")
	}
	if _, ok := allowedCardStyles[t.CardStyle]; !ok {
		return invalidSettings("unknown card style %q", t.CardStyle)
	}
	if _, ok := allowedFonts[t.Font]; !ok {
		return invalidSettings("unknown font %q", t.Font)
	}

	if t.Layout == "" {
		s.Theme.Layout = "default"
		t.Layout = s.Theme.Layout
	}

	if _, ok := allowedLayouts[t.Layout]; !ok {
		return invalidSettings("unknown layout %q", t.Layout)
	}

	if s.Theme.LinkStyle == "" {
		s.Theme.LinkStyle = "buttons"
	}
	t.LinkStyle = s.Theme.LinkStyle

	if _, ok := allowedLinkStyles[t.LinkStyle]; !ok {
		return invalidSettings("unknown link style %q", t.LinkStyle)
	}

	if s.Links == nil {
		s.Links = []dto.Link{}
	}
	if len(s.Links) > maxLinks {
		return invalidSettings("at most %d links", maxLinks)
	}
	for i, l := range s.Links {
		if _, ok := allowedLinkTypes[l.Type]; !ok {
			return invalidSettings("unknown link type %q", l.Type)
		}
		if err := validateLinkURL(l.URL); err != nil {
			return invalidSettings("link %d: %v", i+1, err)
		}
	}

	if s.Blocks == nil {
		s.Blocks = []dto.Block{}
	}
	seen = map[string]struct{}{}
	for _, b := range s.Blocks {
		if _, ok := allowedBlocks[b.Type]; !ok {
			return invalidSettings("unknown block %q", b.Type)
		}
		if _, dup := seen[b.Type]; dup {
			return invalidSettings("duplicate block %q", b.Type)
		}
		seen[b.Type] = struct{}{}
	}

	return nil
}

// validateLinkURL: https only, with a host, no credentials. Blocks javascript:, data:, http:// etc.
func validateLinkURL(raw string) error {
	if len(raw) > maxLinkURLLength {
		return fmt.Errorf("url is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("url must start with https://")
	}
	return nil
}

func blockEnabled(s dto.PageSettings, blockType string) bool {
	for _, b := range s.Blocks {
		if b.Type == blockType {
			return b.Enabled
		}
	}
	return false
}

// withMedia turns the joined upload columns into files.
func (ps *PageService) withMedia(p dto.PageItem) dto.PageItem {
	p.Avatar = media.Build(ps.urls, p.Media.Avatar)
	p.BackgroundMedia = media.Build(ps.urls, p.Media.Background)
	p.Music = media.Build(ps.urls, p.Media.Music)
	return p
}

// GetPublicPage returns the page by slug. Unpublished pages (and pages of banned authors) are
// visible only to the owner; everyone else gets ErrPageNotFound — the same answer as for a missing page.
func (ps *PageService) GetPublicPage(ctx context.Context, viewerID, slug string) (*dto.PublicPage, error) {
	const op = "internal/features/page/service GetPublicPage"

	log := ps.log.With(zap.String("op", op), zap.String("slug", slug))

	slug = normalizeSlug(slug)
	if !slugRe.MatchString(slug) {
		return nil, sharederrors.ErrPageNotFound
	}

	row, err := ps.storage.GetPageBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPageNotFound) {
			return nil, err
		}
		log.Error("get page by slug failed", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	isOwner := viewerID != "" && viewerID == row.UserID
	if !isOwner && (!row.IsPublished || row.AuthorBlocked) {
		return nil, sharederrors.ErrPageNotFound
	}

	item := ps.withMedia(row.PageItem)
	page := &dto.PublicPage{
		Slug:            item.Slug,
		DisplayName:     item.DisplayName,
		Bio:             item.Bio,
		Settings:        item.Settings,
		AuthorUsername:  row.AuthorUsername,
		Avatar:          item.Avatar,
		BackgroundMedia: item.BackgroundMedia,
		Music:           item.Music,
	}

	if blockEnabled(item.Settings, "referral_button") && row.OwnReferralKey != nil {
		page.ReferralKey = row.OwnReferralKey
		if ps.baseurl != "" {
			link := ps.baseurl + "/auth?ref=" + url.QueryEscape(*row.OwnReferralKey)
			page.ReferralLink = &link
		}
	}

	if blockEnabled(item.Settings, "referral_counter") {
		n, err := ps.storage.CountReferrals(ctx, row.UserID)
		if err != nil {
			log.Error("count referrals failed", zap.Error(err))
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		page.ReferralCount = &n
	}

	if blockEnabled(item.Settings, "view_counter") {
		v, err := ps.storage.GetViewStats(ctx, row.UserID, today())
		if err != nil {
			log.Error("view stats failed", zap.Error(err))
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		page.ViewCount = &v.Total
	}

	if blockEnabled(item.Settings, "recent_posts") {
		posts, err := ps.storage.GetRecentPostPreviews(ctx, row.UserID, recentPostsOnPage)
		if err != nil {
			log.Error("recent posts failed", zap.Error(err))
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		page.RecentPosts = posts
	}

	switch {
	case isOwner:
		page.Viewer = &dto.ViewerContext{IsOwner: true}
	case viewerID != "":
		vc, err := ps.storage.GetViewerContext(ctx, viewerID, row.UserID)
		switch {
		case err == nil:
			page.Viewer = &vc
		case errors.Is(err, sharederrors.ErrUserNotFound):
			// deleted after the token was issued -> anonymous
		default:
			log.Error("viewer context failed", zap.Error(err))
			return nil, fmt.Errorf("%s:%w", op, err)
		}
	}

	return page, nil
}

func (ps *PageService) GetMyPage(ctx context.Context, userID string) (dto.PageItem, error) {
	const op = "internal/features/page/service GetMyPage"

	p, err := ps.storage.GetPageByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPageNotFound) {
			return dto.PageItem{}, err
		}
		ps.log.Error("get my page failed", zap.String("op", op), zap.String("user_id", userID), zap.Error(err))
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}

	// the owner always sees the numbers, whether or not the counter is shown on the page
	views, err := ps.storage.GetViewStats(ctx, userID, today())
	if err != nil {
		ps.log.Error("view stats failed", zap.String("op", op), zap.String("user_id", userID), zap.Error(err))
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}
	p.Views = &views

	return ps.withMedia(p), nil
}

func (ps *PageService) CreatePage(ctx context.Context, userID string, req dto.CreatePageRequest) (dto.PageItem, error) {
	const op = "internal/features/page/service CreatePage"

	slug := normalizeSlug(req.Slug)
	if err := validateSlug(slug); err != nil {
		return dto.PageItem{}, err
	}

	var displayName *string
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if utf8.RuneCountInString(name) > maxDisplayNameLength {
			return dto.PageItem{}, invalidSettings("display name is too long (max %d)", maxDisplayNameLength)
		}
		if name != "" {
			displayName = &name
		}
	}

	p, err := ps.storage.CreatePage(ctx, userID, slug, displayName, defaultSettings())
	if err != nil {
		if errors.Is(err, sharederrors.ErrPageAlreadyExists) || errors.Is(err, sharederrors.ErrSlugTaken) {
			return dto.PageItem{}, err
		}
		ps.log.Error("create page failed", zap.String("op", op), zap.String("user_id", userID), zap.Error(err))
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}
	return ps.withMedia(p), nil
}

// checkAttachment validates an upload id from the request: nil = no change, "" = detach.
func (ps *PageService) checkAttachment(ctx context.Context, userID string, uploadID *string, purpose string) error {
	if uploadID == nil || *uploadID == "" {
		return nil
	}
	if _, err := uuid.Parse(*uploadID); err != nil {
		return sharederrors.ErrInvalidMediaAttachment
	}
	ok, err := ps.storage.IsUploadAttachable(ctx, *uploadID, userID, purpose)
	if err != nil {
		return err
	}
	if !ok {
		return sharederrors.ErrInvalidMediaAttachment
	}
	return nil
}

func (ps *PageService) UpdateMyPage(ctx context.Context, userID string, req dto.UpdatePageRequest) (dto.PageItem, error) {
	const op = "internal/features/page/service UpdateMyPage"

	log := ps.log.With(zap.String("op", op), zap.String("user_id", userID))

	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if utf8.RuneCountInString(name) > maxDisplayNameLength {
			return dto.PageItem{}, invalidSettings("display name is too long (max %d)", maxDisplayNameLength)
		}
		req.DisplayName = &name
	}
	if req.Bio != nil {
		bio := strings.TrimSpace(*req.Bio)
		if utf8.RuneCountInString(bio) > maxBioLength {
			return dto.PageItem{}, invalidSettings("bio is too long (max %d)", maxBioLength)
		}
		req.Bio = &bio
	}
	if req.Settings != nil {
		if err := validateSettings(req.Settings); err != nil {
			return dto.PageItem{}, err
		}
	}

	for _, a := range []struct {
		id      *string
		purpose string
	}{{req.BackgroundUploadID, purposeBackground}, {req.MusicUploadID, purposeMusic}} {
		if err := ps.checkAttachment(ctx, userID, a.id, a.purpose); err != nil {
			if errors.Is(err, sharederrors.ErrInvalidMediaAttachment) {
				return dto.PageItem{}, err
			}
			log.Error("check attachment failed", zap.Error(err))
			return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
		}
	}

	// remember the current files: once replaced / detached, they are deleted
	var before dto.PageItem
	if req.BackgroundUploadID != nil || req.MusicUploadID != nil {
		cur, err := ps.storage.GetPageByUserID(ctx, userID)
		if err != nil {
			if errors.Is(err, sharederrors.ErrPageNotFound) {
				return dto.PageItem{}, err
			}
			log.Error("get page failed", zap.Error(err))
			return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
		}
		before = cur
	}

	p, err := ps.storage.UpdatePage(ctx, userID, req.DisplayName, req.Bio, req.Settings, req.IsPublished,
		req.BackgroundUploadID, req.MusicUploadID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPageNotFound) || errors.Is(err, sharederrors.ErrInvalidMediaAttachment) {
			return dto.PageItem{}, err
		}
		log.Error("update page failed", zap.Error(err))
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}

	// page_background / page_music uploads belong to this page only -> free the space
	delCtx := context.WithoutCancel(ctx)
	for _, c := range []struct{ previous, next *string }{
		{before.BackgroundUploadID, req.BackgroundUploadID},
		{before.MusicUploadID, req.MusicUploadID},
	} {
		if replaced(c.previous, c.next) {
			if err := ps.uploads.DeleteUpload(delCtx, userID, *c.previous); err != nil {
				log.Warn("unable to delete previous page file", zap.String("upload_id", *c.previous), zap.Error(err))
			}
		}
	}

	return ps.withMedia(p), nil
}

func (ps *PageService) ChangeSlug(ctx context.Context, userID string, req dto.ChangeSlugRequest) error {
	const op = "internal/features/page/service ChangeSlug"

	slug := normalizeSlug(req.Slug)
	if err := validateSlug(slug); err != nil {
		return err
	}

	current, err := ps.storage.GetPageByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPageNotFound) {
			return err
		}
		return fmt.Errorf("%s:%w", op, err)
	}
	if current.Slug == slug {
		return nil
	}
	if current.SlugChangedAt != nil && time.Since(*current.SlugChangedAt) < ps.slugCooldown {
		return sharederrors.ErrSlugChangeCooldown
	}

	if err := ps.storage.ChangeSlug(ctx, userID, slug); err != nil {
		if errors.Is(err, sharederrors.ErrSlugTaken) || errors.Is(err, sharederrors.ErrPageNotFound) {
			return err
		}
		ps.log.Error("change slug failed", zap.String("op", op), zap.String("user_id", userID), zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}
	return nil
}

func (ps *PageService) CheckSlug(ctx context.Context, slug string) (dto.SlugAvailabilityResponse, error) {
	const op = "internal/features/page/service CheckSlug"

	slug = normalizeSlug(slug)
	if err := validateSlug(slug); err != nil {
		return dto.SlugAvailabilityResponse{Slug: slug, Available: false, Message: err.Error()}, nil
	}

	taken, err := ps.storage.IsSlugTaken(ctx, slug)
	if err != nil {
		return dto.SlugAvailabilityResponse{}, fmt.Errorf("%s:%w", op, err)
	}
	if taken {
		return dto.SlugAvailabilityResponse{Slug: slug, Available: false, Message: sharederrors.ErrSlugTaken.Error()}, nil
	}
	return dto.SlugAvailabilityResponse{Slug: slug, Available: true, Message: "available"}, nil
}

// ===== views =====

// non-browsers and link previews (Telegram, Discord, ...) — they don't open pages for a person
var botUA = regexp.MustCompile(`(?i)bot|crawl|spider|slurp|preview|facebookexternalhit|embedly|whatsapp|skype|curl|wget|python|go-http-client|okhttp|headless|lighthouse`)

// today is the counting day (UTC, so every server instance agrees).
func today() time.Time {
	y, m, d := time.Now().UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// visitorHash identifies a visitor for one day only. The day is part of the HMAC input, so
// tomorrow's hash for the same person is unrelated, and without viewSecret it can't be recomputed.
// The IP itself is never stored.
func (ps *PageService) visitorHash(day time.Time, viewerID, ip, userAgent string) []byte {
	key := "anon|" + ip + "|" + userAgent
	if viewerID != "" {
		key = "user|" + viewerID // the same person on phone and laptop counts once
	}
	mac := hmac.New(sha256.New, ps.viewSecret)
	mac.Write([]byte("page-view|" + day.Format(time.DateOnly) + "|" + key))
	return mac.Sum(nil)[:16]
}

// RecordView counts a visit: once per visitor per day, not the owner, not bots, only published pages.
// Called by the frontend after the page is actually shown (bots that don't run JS never get here).
func (ps *PageService) RecordView(ctx context.Context, slug, viewerID, ip, userAgent string) error {
	const op = "internal/features/page/service RecordView"

	if userAgent == "" || botUA.MatchString(userAgent) {
		return nil
	}
	slug = normalizeSlug(slug)
	if !slugRe.MatchString(slug) {
		return sharederrors.ErrPageNotFound
	}

	p, err := ps.storage.GetPageForView(ctx, slug)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPageNotFound) {
			return err
		}
		return fmt.Errorf("%s:%w", op, err)
	}
	if !p.IsPublished || p.AuthorBlocked {
		return sharederrors.ErrPageNotFound
	}
	if viewerID == p.UserID {
		return nil // the owner checking their own page
	}

	day := today()
	if _, err := ps.storage.RecordView(ctx, p.UserID, day, ps.visitorHash(day, viewerID, ip, userAgent)); err != nil {
		ps.log.Error("record view failed", zap.String("op", op), zap.String("slug", slug), zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}
	return nil
}

// CleanupViewVisitors drops dedupe rows of past days (run by the queue). Yesterday is kept so a
// visit right around midnight UTC isn't counted twice by a lagging clock.
func (ps *PageService) CleanupViewVisitors(ctx context.Context) (int, error) {
	return ps.storage.DeleteViewVisitorsBefore(ctx, today().AddDate(0, 0, -1))
}
