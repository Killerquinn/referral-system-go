package repo

// TODO: write the queries. Placeholders below only document the expected params.
const (
	// $1 user_id, $2 slug, $3 display_name (nullable), $4 settings (jsonb)
	InsertPage = ``

	// $1 user_id
	SelectPageByUserID = ``

	// $1 slug — page + owner: users.id, username, own_referral_key, is_blocked
	SelectPageBySlug = ``

	// $1 user_id, $2 display_name (nullable, "" -> NULL), $3 bio (nullable, "" -> NULL),
	// $4 settings (nullable jsonb), $5 is_published (nullable); updated_at = NOW()
	UpdatePageByUserID = ``

	// $1 user_id, $2 new slug; slug_changed_at = NOW(), updated_at = NOW()
	UpdatePageSlug = ``

	// $1 slug — EXISTS
	SelectSlugExists = ``

	// $1 author_id — current referrals only (users.referred_by_id = author), not history from referrals table
	CountReferralsOfAuthor = ``

	// $1 author_id, $2 limit — id, title, created_at ORDER BY created_at DESC
	SelectRecentPostPreviews = ``

	// $1 viewer_id, $2 author_id — is_referral (viewer.referred_by_id = author),
	// referred_since (referrals.referral_timestamp), has_other_referrer
	SelectViewerContext = ``
)
