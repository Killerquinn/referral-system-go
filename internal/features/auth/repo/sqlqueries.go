package repopackage

const (
	selectIfUserExist = `
	SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`

	selectIfUserBanned = `
	SELECT is_blocked FROM user_sessions WHERE user_id = $1
	`

	dropAllUserSessions = `
	DELETE FROM user_sessions WHERE user_id = $1
	`

	dropSessionByRefreshHash = `
	DELETE FROM user_sessions WHERE refresh_token_hash = $1
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
	UPDATE user_sessions s
	SET refresh_token_hash = $2, expires_at = $3, user_agent = $4, client_ip = $5
	FROM users u
	WHERE s.refresh_token_hash = $1 AND s.expires_at > now()
		AND u.id = s.user_id AND u.is_blocked IS NOT TRUE
	RETURNING s.user_id
	`

	deleteExpiredSessions = `
	DELETE FROM user_sessions
	WHERE expires_at < now()
	`

	//$1 - userID, $2 - purpose('email_verify', 'password_recovery'), $3 - code_hash $4 - expires_at
	codeUpsert = `
	INSERT INTO auth_codes(user_id, purpose, code_hash, expires_at)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (user_id, purpose) DO UPDATE
	SET code_hash = EXCLUDED.code_hash, expires_at = EXCLUDED.expires_at, attempts = 0, created_at = now(), locked_until = NULL
	WHERE (auth_codes.locked_until IS NULL OR auth_codes.locked_until < now()) AND auth_codes.created_at < now() - interval '60 seconds'
	`

	//$1 - userID, $2 - purpose('email_verify', 'password_recovery')
	selectCodeForUpdate = `
	SELECT code_hash, expires_at, attempts, locked_until
	FROM auth_codes
	WHERE user_id = $1 AND purpose = $2
	FOR UPDATE
	`

	//$1 - userID, $2 - hashed_password
	updatePassword = `
	UPDATE users
	SET hashed_password = $2 WHERE id = $1
	`

	//$1 - userID, $2 - purpose('email_verify', 'password_recovery')
	deleteCode = `
	DELETE FROM auth_codes WHERE user_id = $1 AND purpose = $2
	`

	//$1 - userID, $2 - purpose('email_verify', 'password_recovery'), $3 - maxAttempts
	processAttemptQuery = `
	UPDATE auth_codes
	SET attempts = attempts + 1,
		expires_at = CASE WHEN attempts + 1 >= $3 THEN now() ELSE expires_at END,
		locked_until = CASE WHEN attempts + 1 >= $3 THEN now() + interval '2 minutes' ELSE locked_until END
	WHERE user_id = $1 AND purpose = $2
	`

	// AND ... IS NULL: verifying twice keeps the first timestamp
	markEmailVerified = `
	UPDATE users SET email_verified_at = now()
	WHERE id = $1 AND email_verified_at IS NULL
	`

	selectEmailStatus = `
	SELECT email, email_verified_at FROM users WHERE id = $1
	`
)
