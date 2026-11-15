package repo

import "github.com/killerquinn/referral-system-go/internal/shared/media"

const (
	SelectHashedPassword = `
	SELECT hashed_password FROM users WHERE id = $1
	`
	UpdateCurrentPassword = `
	UPDATE users 
	SET hashed_password = $1
	WHERE id = $2
	`

	DeleteAllUserSessions = `
	DELETE FROM user_sessions WHERE user_id = $1
	`

	CheckIfAbleToChangeReferrer = `
	SELECT last_time_ref_used FROM users WHERE id = $1
	`

	CheckOnSelfReferralAndFindReferrerID = `
	SELECT 
    u.id AS user_id,
    ref.id AS referrer_id,
    ref.referred_by_id AS referrer_of_referrer
	FROM users u
	LEFT JOIN users ref ON ref.own_referral_key = $2
	WHERE u.id = $1
	`

	UpdateUsersReferrer = `
	WITH updated_user AS (
    UPDATE users
    SET 
        referred_by_id = $1,
        last_time_ref_used = $2
    WHERE id = $3
	RETURNING id
	)
	INSERT INTO referrals (referral_owner, referred_user_id, referral_timestamp)
	SELECT $1, $3, $2
	FROM updated_user
	ON CONFLICT (referral_owner, referred_user_id) 
	DO UPDATE SET referral_timestamp = EXCLUDED.referral_timestamp
	`

	SelectIfExist = `
	SELECT EXISTS(
		SELECT 1
		FROM users
		WHERE own_referral_key = $1
	)
	`

	DeleteUser = `
	DELETE
	FROM users 
	WHERE id = $1
	`

	UpdateReferralsCooldown = `
	UPDATE users
	SET last_time_ref_used = NULL
	WHERE referred_by_id = $1
	`

	WriteUniqueRefKey = `
	UPDATE users
	SET own_referral_key = $2
	WHERE id = $1
 	AND own_referral_key IS NULL
  	AND NOT EXISTS (SELECT 1 FROM users WHERE own_referral_key = $2)
	RETURNING own_referral_key
	`
)

// vars, not consts: the avatar columns come from media.JoinColumns (a function call).
var (
	SelectUser = `
	SELECT
		u.username,
		ref.username AS referrer_username,
		u.own_referral_key,
		CASE
			WHEN u.referred_by_id = $1 THEN u.last_time_ref_used
			ELSE NULL
		END AS last_time_ref_used,
		u.is_blocked,
		u.created_at,
		` + media.JoinColumns("av") + `
	FROM users u
	LEFT JOIN users ref ON u.referred_by_id = ref.id
	LEFT JOIN uploads av ON ` + media.JoinOn("av", "u.avatar_upload_id") + `
	WHERE u.username = $2
`

	SelectSelf = `
	SELECT
		u.username,
		u.email,
		u.own_referral_key,
		(SELECT COUNT(*) FROM users r WHERE r.referred_by_id = u.id) AS referrals_count,
		ref.username AS referrer_username,
		u.last_time_ref_used,
		u.is_blocked,
		u.created_at,
		(SELECT MAX(us.created_at) FROM user_sessions us WHERE us.user_id = u.id) AS last_login,
		` + media.JoinColumns("av") + `
	FROM users u
	LEFT JOIN users ref ON ref.id = u.referred_by_id
	LEFT JOIN uploads av ON ` + media.JoinOn("av", "u.avatar_upload_id") + `
	WHERE u.id = $1
	`

	// $1 user_id, $2 upload_id (NULL = remove the avatar).
	// The upload must be the user's own, ready, an avatar, in the public bucket — checked in the same
	// statement, so there is no gap between "check" and "set". No row back -> invalid upload.
	// Returns the PREVIOUS avatar id first (CTEs see the row as it was before the UPDATE),
	// so the service can delete the old file.
	SetAvatar = `
	WITH old AS (
		SELECT avatar_upload_id AS old_id FROM users WHERE id = $1
	), upd AS (
		UPDATE users
		SET avatar_upload_id = $2
		WHERE id = $1
		  AND ($2::uuid IS NULL OR EXISTS (
			SELECT 1 FROM uploads
			WHERE id = $2 AND owner_id = $1 AND status = 'ready' AND purpose = 'avatar' AND bucket = 'public'
		  ))
		RETURNING avatar_upload_id
	)
	SELECT (SELECT old_id FROM old), ` + media.JoinColumns("av") + `
	FROM upd
	LEFT JOIN uploads av ON ` + media.JoinOn("av", "upd.avatar_upload_id") + `
	`
)
