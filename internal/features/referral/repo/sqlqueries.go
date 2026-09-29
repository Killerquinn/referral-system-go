package repo

const (
	FindReferrerByReferralUsername = `
	SELECT 
		reffered_by,
		last_time_ref_used
	FROM
		users
	WHERE
		username = $1
	`

	GetReferrerByUserID = `
	SELECT 
		username
	FROM
		users
	WHERE
		id = $1
	`

	GetReferralsByUsername = `
	SELECT 
    	referred.id AS id,
    	referred.username AS username,
    	r.referral_timestamp AS joined_at
	FROM referrals r
	JOIN users owner ON owner.id = r.referral_owner
	JOIN users referred ON referred.id = r.referred_user_id
	WHERE owner.username = $1
  	AND (
    	$2::timestamptz IS NULL
    OR (r.referral_timestamp, r.referred_user_id) < ($2, $3)
  	)
	ORDER BY r.referral_timestamp DESC, r.referred_user_id DESC
	LIMIT $4
	`

	GetReferralsByOwnUserID = `
	SELECT 
    	referred.id AS id,
    	referred.username AS username,
    	r.referral_timestamp AS joined_at
	FROM referrals r
	JOIN users owner ON owner.id = r.referral_owner
	JOIN users referred ON referred.id = r.referred_user_id
	WHERE owner.id = $1
  	AND (
    	$2::timestamptz IS NULL
    OR (r.referral_timestamp, r.referred_user_id) < ($2, $3)
  	)
	ORDER BY r.referral_timestamp DESC, r.referred_user_id DESC
	LIMIT $4
	`

	CheckIfUserIsBanned = `
	SELECT
		is_blocked
	FROM
		user_sessions
	WHERE id = $1
	`

	GetQuantityOfReferrers = `
	SELECT COUNT(*)
	FROM referrals
	WHERE referral_owner = $1
  		AND ($2::timestamptz IS NULL OR referral_timestamp >= $2)
  		AND ($3::timestamptz IS NULL OR referral_timestamp <= $3);
	`

	GetRandomUsersByOffset = `
	SELECT 
    	w.referred_user_id AS id,
    	w.username,
    	w.joined_at
	FROM UNNEST($1::int[]) WITH ORDINALITY AS o(offset_val, ord)
	CROSS JOIN LATERAL (
    SELECT 
        r.referred_user_id,
        u.username,
        r.referral_timestamp AS joined_at
    FROM referrals r
    JOIN users u ON u.id = r.referred_user_id
    WHERE r.referral_owner = $2
      AND ($3::timestamptz IS NULL OR r.referral_timestamp >= $3)
      AND ($4::timestamptz IS NULL OR r.referral_timestamp <= $4)
    ORDER BY r.referral_timestamp ASC, r.referred_user_id ASC
    LIMIT 1 OFFSET o.offset_val
	) w
	ORDER BY o.ord;
	`
)
