package dto

import (
	"time"

	"github.com/killerquinn/referral-system-go/internal/shared/media"
)

type (
	// ===== settings (stored in profile_pages.settings JSONB) =====
	// Typed on purpose: unknown JSON keys are dropped on decode, every value is validated in the service.
	// No user CSS/HTML/JS ever — only choices from fixed sets (see allowed* vars in the service).

	PageSettings struct {
		Background Background `json:"background"`
		Effects    []string   `json:"effects"` // from effectGroups, limited per group
		Theme      Theme      `json:"theme"`
		Links      []Link     `json:"links"`
		Blocks     []Block    `json:"blocks"` // order in the slice = order on the page
	}

	Background struct {
		Type     string    `json:"type"`               // "color" | "gradient" | "media" (the page's background upload)
		Color    *string   `json:"color,omitempty"`    // hex, for type "color"
		Gradient *Gradient `json:"gradient,omitempty"` // for type "gradient"
	}

	Gradient struct {
		From  string `json:"from"`  // hex
		To    string `json:"to"`    // hex
		Angle int    `json:"angle"` // 0-360
	}

	Theme struct {
		AccentColor string `json:"accent_color"` // hex
		CardStyle   string `json:"card_style"`   // from allowedCardStyles
		Font        string `json:"font"`         // from allowedFonts
		Layout      string `json:"layout"`
		LinkStyle   string `json:"link_style"`
	}

	Link struct {
		Type string `json:"type"` // from allowedLinkTypes
		URL  string `json:"url"`  // https:// only
	}

	Block struct {
		Type    string `json:"type"` // from allowedBlocks
		Enabled bool   `json:"enabled"`
	}

	// ===== entity (owner / editor view) =====

	PageItem struct {
		Slug          string       `json:"slug"`
		DisplayName   *string      `json:"display_name,omitempty"`
		Bio           *string      `json:"bio,omitempty"`
		Settings      PageSettings `json:"settings"`
		IsPublished   bool         `json:"is_published"`
		SlugChangedAt *time.Time   `json:"slug_changed_at,omitempty"`
		CreatedAt     time.Time    `json:"created_at"`
		UpdatedAt     *time.Time   `json:"updated_at,omitempty"`

		// attached files: ids for the editor, resolved files for the preview
		BackgroundUploadID *string     `json:"background_upload_id,omitempty"`
		MusicUploadID      *string     `json:"music_upload_id,omitempty"`
		Avatar             *media.File `json:"avatar,omitempty"` // the owner's avatar (set in settings, not here)
		BackgroundMedia    *media.File `json:"background_media,omitempty"`
		Music              *media.File `json:"music,omitempty"`

		Media PageMediaRows `json:"-"` // filled by the repo, turned into the files above by the service

		Views *PageViews `json:"views,omitempty"` // owner only (GetMyPage)
	}

	// PageViews are unique visitors: one per visitor per day.
	PageViews struct {
		Total int `json:"total"`
		Today int `json:"today"`
	}

	// PageForView is the little the view counter needs about a page.
	PageForView struct {
		UserID        string
		IsPublished   bool
		AuthorBlocked bool
	}

	// PageMediaRows are the joined uploads columns for the files a page shows.
	PageMediaRows struct {
		Avatar     media.Row
		Background media.Row
		Music      media.Row
	}

	// PageWithAuthor is a repo-level row for the public page: the page + what we need about its owner.
	PageWithAuthor struct {
		PageItem
		UserID         string
		AuthorUsername string
		OwnReferralKey *string
		AuthorBlocked  bool
	}

	// ===== public page (what a visitor sees) =====

	PostPreview struct {
		ID        string    `json:"id" db:"id"`
		Title     string    `json:"title" db:"title"`
		CreatedAt time.Time `json:"created_at" db:"created_at"`
	}

	// ViewerContext is filled only for a logged-in visitor (nil for anonymous).
	ViewerContext struct {
		IsOwner          bool       `json:"is_owner"`
		IsReferral       bool       `json:"is_referral"` // viewer's current referrer is the page owner
		ReferredSince    *time.Time `json:"referred_since,omitempty"`
		HasOtherReferrer bool       `json:"has_other_referrer"` // -> frontend shows "change referrer" instead of "join"
	}

	PublicPage struct {
		Slug           string       `json:"slug"`
		DisplayName    *string      `json:"display_name,omitempty"`
		Bio            *string      `json:"bio,omitempty"`
		Settings       PageSettings `json:"settings"`
		AuthorUsername string       `json:"author_username"`
		ReferralKey    *string      `json:"referral_key,omitempty"`  // nil if the author has no referral key yet
		ReferralLink   *string      `json:"referral_link,omitempty"` // baseurl/auth?ref=<key>, nil without a key

		Avatar          *media.File `json:"avatar,omitempty"`
		BackgroundMedia *media.File `json:"background_media,omitempty"` // used when settings.background.type = "media"
		Music           *media.File `json:"music,omitempty"`

		// block data, nil when the block is disabled
		ReferralCount *int          `json:"referral_count,omitempty"` // number only, never who
		ViewCount     *int          `json:"view_count,omitempty"`     // unique visitors, all time
		RecentPosts   []PostPreview `json:"recent_posts,omitempty"`   // titles + dates only

		Viewer *ViewerContext `json:"viewer,omitempty"`
	}

	PublicPageResponse struct {
		Page    *PublicPage `json:"page,omitempty"`
		Message string      `json:"message"`
	}

	// ===== own page =====

	PageResponse struct {
		Page    *PageItem `json:"page,omitempty"`
		Message string    `json:"message"`
	}

	// ===== create =====

	CreatePageRequest struct {
		Slug        string  `json:"slug"`
		DisplayName *string `json:"display_name,omitempty"`
	}

	// ===== update =====

	// nil field = leave as is; "" for display_name / bio = clear it.
	// settings is replaced as a whole (the editor always sends the full object).
	UpdatePageRequest struct {
		DisplayName *string       `json:"display_name,omitempty"`
		Bio         *string       `json:"bio,omitempty"`
		Settings    *PageSettings `json:"settings,omitempty"`
		IsPublished *bool         `json:"is_published,omitempty"`
		// nil = leave as is, "" = detach, otherwise an id of the user's ready upload with the matching purpose
		BackgroundUploadID *string `json:"background_upload_id,omitempty"`
		MusicUploadID      *string `json:"music_upload_id,omitempty"`
	}

	// ===== slug =====

	ChangeSlugRequest struct {
		Slug string `json:"slug"`
	}

	SlugAvailabilityResponse struct {
		Slug      string `json:"slug"`
		Available bool   `json:"available"`
		Message   string `json:"message"` // why not: taken / reserved / invalid format
	}

	MessageResponse struct {
		Message string `json:"message"`
	}
)
