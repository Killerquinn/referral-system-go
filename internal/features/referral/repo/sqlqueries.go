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
)
