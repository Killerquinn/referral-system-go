package dto

import "time"

type (
	// ===== settings (stored in profile_pages.settings JSONB) =====
	// Typed on purpose: unknown JSON keys are dropped on decode, every value is validated in the service.
	// No user CSS/HTML/JS ever — only choices from fixed sets (see allowed* vars in the service).

	PageSettings struct {
		Background Background `json:"background"`
		Effects    []string   `json:"effects"` // from allowedEffects
		Theme      Theme      `json:"theme"`
		Links      []Link     `json:"links"`
		Blocks     []Block    `json:"blocks"` // order in the slice = order on the page
	}

	Background struct {
		Type     string    `json:"type"`               // "color" | "gradient" (later: "image", "video" after R2)
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
		Slug          string       `json:"slug" db:"slug"`
		DisplayName   *string      `json:"display_name,omitempty" db:"display_name"`
		Bio           *string      `json:"bio,omitempty" db:"bio"`
		Settings      PageSettings `json:"settings" db:"settings"`
		IsPublished   bool         `json:"is_published" db:"is_published"`
		SlugChangedAt *time.Time   `json:"slug_changed_at,omitempty" db:"slug_changed_at"`
		CreatedAt     time.Time    `json:"created_at" db:"created_at"`
		UpdatedAt     *time.Time   `json:"updated_at,omitempty" db:"updated_at"`
	}

	// PageWithAuthor is a repo-level row for the public page: the page + what we need about its owner.
	PageWithAuthor struct {
		PageItem
		UserID         string  `db:"user_id"`
		AuthorUsername string  `db:"author_username"`
		OwnReferralKey *string `db:"own_referral_key"`
		AuthorBlocked  bool    `db:"author_blocked"`
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
		ReferralLink   *string      `json:"referral_link,omitempty"` // nil if the author has no referral key yet

		// block data, nil when the block is disabled
		ReferralCount *int          `json:"referral_count,omitempty"` // number only, never who
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
