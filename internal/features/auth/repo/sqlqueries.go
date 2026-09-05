package repopackage

const (
	selectIfUserExist = `
	SELECT EXISTS(SELECT 1 FROM users WHERE email = '$1')
	`

	createUserQuery = `
	INSERT INTO users(
    username,
    email,
    hashedpassw,
    avatar) VALUES (
	$1, $2, $3, COALESCE(NULLIF($4, ''), null)
	) RETURNING user_id, created_at`
)
