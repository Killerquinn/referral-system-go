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
	)
	ON CONFLICT (username) DO NOTHING
	RETURNING id
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
	rotateSession = `
	UPDATE user_sessions
	SET refresh_token_hash = $2, expires_at = $3, user_agent = $4, client_ip = $5
	WHERE refresh_token_hash = $1 AND expires_at > now()
	RETURNING user_id
	`
)
