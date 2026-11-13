package dto

import "time"

type (
	// ===== entity =====

	TierItem struct {
		ID          string     `json:"id" db:"id"`
		AuthorID    string     `json:"-" db:"author_id"`
		Level       int        `json:"level" db:"level"`
		Name        string     `json:"name" db:"name"`
		Description *string    `json:"description,omitempty" db:"description"`
		PriceAmount int        `json:"price_amount" db:"price_amount"` // minor units (cents)
		Currency    string     `json:"currency" db:"currency"`
		IsArchived  bool       `json:"is_archived" db:"is_archived"`
		CreatedAt   time.Time  `json:"created_at" db:"created_at"`
		UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`
	}

	// ===== create =====

	// level is not sent by the client: a new tier goes last (max(level) + 1)
	CreateTierRequest struct {
		Name        string  `json:"name"`
		Description *string `json:"description,omitempty"`
		PriceAmount int     `json:"price_amount"`
		Currency    string  `json:"currency"`
	}
	CreateTierResponse struct {
		Tier    *TierItem `json:"tier,omitempty"`
		Message string    `json:"message"`
	}

	// ===== update =====

	// nil field = leave as is.
	// Price is not editable on purpose: Stripe prices are immutable (archive + create a new tier instead).
	UpdateTierRequest struct {
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
	}
	UpdateTierResponse struct {
		Message string `json:"message"`
	}

	// ===== archive (DELETE) =====

	ArchiveTierResponse struct {
		Message string `json:"message"`
	}

	// ===== list =====

	TierListResponse struct {
		Items   []TierItem `json:"items"`
		Message string     `json:"message"`
	}
)
