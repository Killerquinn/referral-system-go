package repo

// TODO: write the queries. Placeholders below only document the expected params.
const (
	// $1 author_id, $2 name, $3 description (nullable), $4 price_amount, $5 currency
	// level = COALESCE(MAX(level), 0) + 1 among the author's tiers
	InsertTier = ``

	// $1 author_id — number of not archived tiers
	CountActiveTiersByAuthor = ``

	// $1 tier_id, $2 author_id, $3 name (nullable), $4 description (nullable)
	UpdateTierByIDAndAuthor = ``

	// $1 tier_id, $2 author_id — sets is_archived = TRUE
	ArchiveTierByIDAndAuthor = ``

	// $1 author_id, $2 include_archived (bool), ORDER BY level
	SelectTiersByAuthorID = ``

	// $1 author username — not archived only, ORDER BY level
	SelectTiersByAuthorUsername = ``

	// $1 username — is_blocked of the author
	SelectAuthorBlockedByUsername = ``
)
