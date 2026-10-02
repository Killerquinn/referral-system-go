package repopackage

const (
	selectIfUserExist = `
	SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`

	selectIfUserBanned = `
	SELECT is_blocked FROM user_sessions WHERE user_id = $1
	`

	dropUsersSessionIfExist = `
	DELETE FROM user_sessions WHERE user_id = $1
	`

	getUserByEmail = `
	SELECT
		id,
		username,
		email,
		hashed_password,
		own_referral_key,
		referred_by_id,
		last_time_ref_used,
		is_blocked,
		created_at
	FROM users WHERE email = $1
	`

	createUserQuery = `
	INSERT INTO users(
    username,
    email,
    hashed_password
	) VALUES (
	$1, $2, $3
	) RETURNING id
	`

	postSession = `
	INSERT INTO user_sessions ( 
    user_id, 
    refresh_token_hash, 
    user_agent, 
    client_ip, 
    expires_at
	) VALUES ($1, $2, $3, $4, $5) RETURNING refresh_token_hash
	`
)
