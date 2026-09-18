package repo

const (
	SelectHashedPassword = `
	SELECT hashed_password FROM users WHERE id = $1
	`
	UpdateCurrentPassword = `
	UPDATE users 
	SET hashed_password = $1
	WHERE id = $2
	`

	CheckIfAbleToChangeReferrer = `
	SELECT last_time_ref_used FROM users WHERE id = $1
	`

	CheckOnSelfReferralAndFindReferrerID = `
	SELECT 
    u.id AS user_id,
    u.last_time_ref_used,
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
	UPDATE referrals
	SET
  	  referred_user_id = $3,
  	  referral_timestamp = NOW()
	WHERE referral_owner = $1;
	`
)
