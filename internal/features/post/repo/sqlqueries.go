package repo

const (
	// $1 author_id, $2 title, $3 body
	InsertPost = `
	WITH inserted AS (
		INSERT INTO posts (author_id, title, body)
		VALUES ($1, $2, $3)
		RETURNING id, author_id, title, body, created_at
	)
	SELECT
		i.id,
		i.author_id,
		u.username AS author_username,
		i.title,
		i.body,
		i.created_at
	FROM inserted i
	JOIN users u ON u.id = i.author_id
	`

	// $1 post_id
	SelectPostByID = `
	SELECT
		p.id,
		p.author_id,
		u.username AS author_username,
		p.title,
		p.body,
		p.created_at,
		p.updated_at,
		COALESCE(u.is_blocked, FALSE) AS is_blocked
	FROM posts p
	JOIN users u ON u.id = p.author_id
	WHERE p.id = $1
	`

	// $1 post_id, $2 author_id, $3 title (nullable), $4 body (nullable)
	UpdatePostByIDAndAuthor = `
	UPDATE posts
	SET
		title = COALESCE($3, title),
		body = COALESCE($4, body),
		updated_at = NOW()
	WHERE id = $1
	  AND author_id = $2
	`

	// $1 post_id, $2 author_id
	DeletePostByIDAndAuthor = `
	DELETE FROM posts
	WHERE id = $1
	  AND author_id = $2
	`

	// $1 author_id, $2 cursor created_at (nullable), $3 cursor id (nullable), $4 limit
	SelectPostsByAuthorID = `
	SELECT
		p.id,
		p.author_id,
		u.username AS author_username,
		p.title,
		p.body,
		p.created_at,
		p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id
	WHERE p.author_id = $1
	  AND (
		$2::timestamptz IS NULL
		OR (p.created_at, p.id) < ($2::timestamptz, $3::uuid)
	  )
	ORDER BY p.created_at DESC, p.id DESC
	LIMIT $4
	`

	// $1 author username, $2 cursor created_at (nullable), $3 cursor id (nullable), $4 limit
	SelectPostsByAuthorUsername = `
	SELECT
		p.id,
		p.author_id,
		u.username AS author_username,
		p.title,
		p.body,
		p.created_at,
		p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id
	WHERE u.username = $1
	  AND (
		$2::timestamptz IS NULL
		OR (p.created_at, p.id) < ($2::timestamptz, $3::uuid)
	  )
	ORDER BY p.created_at DESC, p.id DESC
	LIMIT $4
	`

	// $1 reader_id, $2 author username — true if reader is the author, the author's referral,
	// or the author's referrer (current referrer = users.referred_by_id; referrals table also keeps old links)
	CanReadPostsOf = `
	SELECT
		a.id = $1::uuid
		OR a.referred_by_id = $1::uuid
		OR EXISTS (
			SELECT 1
			FROM users r
			WHERE r.id = $1::uuid
			  AND r.referred_by_id = a.id
		) AS can_read
	FROM users a
	WHERE a.username = $2
	`

	// $1 username — is_blocked of the author
	SelectAuthorBlockedByUsername = `
	SELECT COALESCE(is_blocked, FALSE)
	FROM users
	WHERE username = $1
	`

	// $1 user_id — current referrer of the user (NULLs if not referred)
	SelectCurrentReferrer = `
	SELECT
		ref.id,
		COALESCE(ref.is_blocked, FALSE)
	FROM users u
	LEFT JOIN users ref ON ref.id = u.referred_by_id
	WHERE u.id = $1
	`
)
