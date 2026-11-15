package repo

import "github.com/killerquinn/referral-system-go/internal/shared/media"

// pageColumns — scanned by scanPage in this order.
const pageColumns = `p.slug, p.display_name, p.bio, p.settings, p.is_published, p.slug_changed_at,
	p.created_at, p.updated_at, p.background_upload_id, p.music_upload_id`

// the three files a page shows; only ready, public uploads come back (see media.JoinOn)
var mediaColumns = media.JoinColumns("av") + ",\n\t" + media.JoinColumns("bg") + ",\n\t" + media.JoinColumns("mu")

var mediaJoins = `
	LEFT JOIN uploads av ON ` + media.JoinOn("av", "u.avatar_upload_id") + `
	LEFT JOIN uploads bg ON ` + media.JoinOn("bg", "p.background_upload_id") + `
	LEFT JOIN uploads mu ON ` + media.JoinOn("mu", "p.music_upload_id")

// vars, not consts: built from the media helpers.
var (
	// $1 user_id, $2 slug, $3 display_name (nullable), $4 settings (jsonb)
	InsertPage = `
	INSERT INTO profile_pages (user_id, slug, display_name, settings)
	VALUES ($1, $2, $3, $4)`

	// $1 user_id
	SelectPageByUserID = `
	SELECT ` + pageColumns + `,
	` + mediaColumns + `
	FROM profile_pages p
	JOIN users u ON u.id = p.user_id` + mediaJoins + `
	WHERE p.user_id = $1`

	// $1 slug — page + owner
	SelectPageBySlug = `
	SELECT ` + pageColumns + `,
	` + mediaColumns + `,
	u.id, u.username, u.own_referral_key, COALESCE(u.is_blocked, FALSE)
	FROM profile_pages p
	JOIN users u ON u.id = p.user_id` + mediaJoins + `
	WHERE p.slug = $1`

	// $1 user_id, $2 display_name, $3 bio, $4 settings, $5 is_published, $6 background_upload_id, $7 music_upload_id.
	// NULL param = leave the column as is; "" for text / upload ids = clear it.
	UpdatePageByUserID = `
	UPDATE profile_pages SET
		display_name = CASE WHEN $2::text IS NULL THEN display_name ELSE NULLIF($2::text, '') END,
		bio = CASE WHEN $3::text IS NULL THEN bio ELSE NULLIF($3::text, '') END,
		settings = COALESCE($4::jsonb, settings),
		is_published = COALESCE($5::boolean, is_published),
		background_upload_id = CASE WHEN $6::text IS NULL THEN background_upload_id ELSE NULLIF($6::text, '')::uuid END,
		music_upload_id = CASE WHEN $7::text IS NULL THEN music_upload_id ELSE NULLIF($7::text, '')::uuid END,
		updated_at = NOW()
	WHERE user_id = $1`

	// $1 user_id, $2 new slug
	UpdatePageSlug = `
	UPDATE profile_pages
	SET slug = $2, slug_changed_at = NOW(), updated_at = NOW()
	WHERE user_id = $1`

	// $1 slug
	SelectSlugExists = `SELECT EXISTS (SELECT 1 FROM profile_pages WHERE slug = $1)`

	// $1 upload_id, $2 owner_id, $3 purpose — can this upload be shown on the owner's page?
	SelectUploadAttachable = `
	SELECT EXISTS (
		SELECT 1 FROM uploads
		WHERE id = $1 AND owner_id = $2 AND purpose = $3 AND status = 'ready' AND bucket = 'public'
	)`

	// $1 author_id — current referrals only (users.referred_by_id), not the history in referrals
	CountReferralsOfAuthor = `SELECT COUNT(*) FROM users WHERE referred_by_id = $1`

	// $1 author_id, $2 limit
	SelectRecentPostPreviews = `
	SELECT id, title, created_at
	FROM posts
	WHERE author_id = $1
	ORDER BY created_at DESC, id DESC
	LIMIT $2`

	// $1 viewer_id, $2 author_id. No row = viewer doesn't exist anymore (deleted after the token was issued).
	SelectViewerContext = `
	SELECT
		COALESCE(v.referred_by_id = $2, FALSE) AS is_referral,
		(SELECT r.referral_timestamp FROM referrals r
		 WHERE r.referral_owner = $2 AND r.referred_user_id = $1 AND v.referred_by_id = $2) AS referred_since,
		(v.referred_by_id IS NOT NULL AND v.referred_by_id <> $2) AS has_other_referrer
	FROM users v
	WHERE v.id = $1`

	// ===== views =====

	// $1 slug
	SelectPageForView = `
	SELECT p.user_id, p.is_published, COALESCE(u.is_blocked, FALSE)
	FROM profile_pages p
	JOIN users u ON u.id = p.user_id
	WHERE p.slug = $1`

	// $1 page_user_id, $2 day, $3 visitor_hash. 0 rows = already counted today.
	InsertViewVisitor = `
	INSERT INTO page_view_visitors (page_user_id, day, visitor_hash)
	VALUES ($1, $2, $3)
	ON CONFLICT DO NOTHING`

	// $1 page_user_id, $2 day
	IncrementDailyViews = `
	INSERT INTO page_view_daily (page_user_id, day, views)
	VALUES ($1, $2, 1)
	ON CONFLICT (page_user_id, day) DO UPDATE SET views = page_view_daily.views + 1`

	// $1 page_user_id, $2 today
	SelectViewStats = `
	SELECT COALESCE(SUM(views), 0), COALESCE(SUM(views) FILTER (WHERE day = $2), 0)
	FROM page_view_daily
	WHERE page_user_id = $1`

	// $1 day — everything before it
	DeleteOldViewVisitors = `DELETE FROM page_view_visitors WHERE day < $1`
)
