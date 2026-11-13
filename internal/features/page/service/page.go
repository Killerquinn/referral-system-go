package service

import (
	"context"
	"time"

	"github.com/killerquinn/referral-system-go/internal/features/page/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type pageStorage interface {
	// unique violation on user_id -> sharederrors.ErrPageAlreadyExists, on slug -> sharederrors.ErrSlugTaken
	CreatePage(ctx context.Context, userID, slug string, displayName *string, settings dto.PageSettings) (dto.PageItem, error)
	GetPageByUserID(ctx context.Context, userID string) (dto.PageItem, error)
	GetPageBySlug(ctx context.Context, slug string) (dto.PageWithAuthor, error)
	UpdatePage(ctx context.Context, userID string, displayName, bio *string, settings *dto.PageSettings, isPublished *bool) (dto.PageItem, error)
	ChangeSlug(ctx context.Context, userID, newSlug string) error
	IsSlugTaken(ctx context.Context, slug string) (bool, error)

	// block data (read-only, the page module doesn't call other features' services)
	CountReferrals(ctx context.Context, authorID string) (int, error)
	GetRecentPostPreviews(ctx context.Context, authorID string, limit int) ([]dto.PostPreview, error)
	GetViewerContext(ctx context.Context, viewerID, authorID string) (dto.ViewerContext, error)
}

type PageService struct {
	log          *zap.Logger
	baseurl      string
	slugCooldown time.Duration
	storage      pageStorage
}

func New(log *zap.Logger, baseurl string, slugCooldown time.Duration, storage pageStorage) *PageService {
	return &PageService{
		log:          log,
		baseurl:      baseurl,
		slugCooldown: slugCooldown,
		storage:      storage,
	}
}

// ===== allowed values for settings (single source of truth, the frontend mirrors them) =====

var (
	allowedBackgroundTypes = map[string]struct{}{"color": {}, "gradient": {}}
	allowedEffects         = map[string]struct{}{"snow": {}, "particles": {}, "rain": {}, "typewriter_bio": {}, "glow_name": {}, "tilt_card": {}}
	allowedCardStyles      = map[string]struct{}{"glass": {}, "solid": {}, "transparent": {}}
	allowedFonts           = map[string]struct{}{"inter": {}, "jetbrains_mono": {}, "space_grotesk": {}, "playfair": {}}
	allowedLinkTypes       = map[string]struct{}{"telegram": {}, "discord": {}, "youtube": {}, "twitch": {}, "tiktok": {}, "instagram": {}, "github": {}, "website": {}}
	// later: "tiers" (after the tier feature), "giveaway" (after giveaways get an API)
	allowedBlocks = map[string]struct{}{"referral_button": {}, "referral_counter": {}, "recent_posts": {}}

	// slugs that clash with frontend/backend routes or look official
	reservedSlugs = map[string]struct{}{
		"api": {}, "auth": {}, "login": {}, "logout": {}, "register": {}, "signup": {},
		"user": {}, "users": {}, "me": {}, "admin": {}, "settings": {}, "social": {},
		"posts": {}, "pages": {}, "page": {}, "slug-available": {}, "static": {}, "assets": {},
		"help": {}, "support": {}, "about": {}, "terms": {}, "privacy": {}, "pricing": {},
	}
)

const (
	maxEffects        = 3
	maxLinks          = 10
	maxLinkURLLength  = 300
	recentPostsOnPage = 5
)

// defaultSettings is what a freshly created page gets.
func defaultSettings() dto.PageSettings {
	// TODO: pleasant default background/theme, blocks: referral_button + referral_counter enabled
	return dto.PageSettings{}
}

// validateSlug: ^[a-z0-9_-]{3,32}$ and not in reservedSlugs.
func validateSlug(slug string) error {
	// TODO: invalid -> sharederrors.ErrInvalidSlug, reserved -> sharederrors.ErrSlugReserved
	return sharederrors.ErrNotImplemented
}

// validateSettings checks every value against the allowed sets / hex format / ranges / https-only URLs,
// limits (maxEffects, maxLinks), no duplicate blocks. Any problem -> sharederrors.ErrInvalidPageSettings (wrap with a reason).
func validateSettings(s dto.PageSettings) error {
	// TODO
	return sharederrors.ErrNotImplemented
}

// GetPublicPage returns the page by slug. Unpublished pages are visible only to the owner.
// Block data is loaded only for enabled blocks; viewer context only for a logged-in viewer.
func (ps *PageService) GetPublicPage(ctx context.Context, viewerID, slug string) (*dto.PublicPage, error) {
	const op = "internal/features/page/service GetPublicPage"
	// TODO: not found / unpublished for non-owner -> sharederrors.ErrPageNotFound,
	// banned author -> sharederrors.ErrUserIsBanned, referral link from OwnReferralKey + baseurl
	return nil, sharederrors.ErrNotImplemented
}

func (ps *PageService) GetMyPage(ctx context.Context, userID string) (dto.PageItem, error) {
	const op = "internal/features/page/service GetMyPage"
	// TODO: no page -> sharederrors.ErrPageNotFound
	return dto.PageItem{}, sharederrors.ErrNotImplemented
}

func (ps *PageService) CreatePage(ctx context.Context, userID string, req dto.CreatePageRequest) (dto.PageItem, error) {
	const op = "internal/features/page/service CreatePage"
	// TODO: normalize slug (trim + lower), validateSlug, create with defaultSettings()
	return dto.PageItem{}, sharederrors.ErrNotImplemented
}

func (ps *PageService) UpdateMyPage(ctx context.Context, userID string, req dto.UpdatePageRequest) (dto.PageItem, error) {
	const op = "internal/features/page/service UpdateMyPage"
	// TODO: validateSettings if req.Settings != nil
	return dto.PageItem{}, sharederrors.ErrNotImplemented
}

func (ps *PageService) ChangeSlug(ctx context.Context, userID string, req dto.ChangeSlugRequest) error {
	const op = "internal/features/page/service ChangeSlug"
	// TODO: validateSlug, SlugChangedAt + slugCooldown > now -> sharederrors.ErrSlugChangeCooldown
	return sharederrors.ErrNotImplemented
}

func (ps *PageService) CheckSlug(ctx context.Context, slug string) (dto.SlugAvailabilityResponse, error) {
	const op = "internal/features/page/service CheckSlug"
	// TODO: validateSlug first (invalid / reserved = not available), then IsSlugTaken
	return dto.SlugAvailabilityResponse{}, sharederrors.ErrNotImplemented
}
