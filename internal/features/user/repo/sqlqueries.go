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
	UPDATE users
	SET referra
	` //todo: <- finish it
)
