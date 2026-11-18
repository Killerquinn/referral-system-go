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
		av.media_type AS author_avatar_type,
		av.storage_key AS author_avatar_key,
		av.variants->>'thumb' AS author_avatar_thumb,
		av.blurhash AS author_avatar_blurhash,
		i.title,
		i.body,
		i.created_at
	FROM inserted i
	JOIN users u ON u.id = i.author_id
	LEFT JOIN uploads av ON av.id = u.avatar_upload_id AND av.status = 'ready' AND av.bucket = 'public'
	`

	// $1 postID, $2 author_id, $3 upload_id's (array)
	InsertMedia = `
	INSERT INTO post_uploads (post_id, upload_id, position)
	SELECT 
	 $1::uuid, 
	 m.id, 
	 m.ord - 1 
	FROM unnest($3::uuid[]) WITH ORDINALITY AS m(id, ord)
	JOIN uploads u ON u.id = m.id 
	AND u.owner_id = $2
	 AND u.status = 'ready'
	  AND u.purpose = 'post_media'
	`

	// $1 post_id — unlink all attachments (before inserting the new list on update)
	DeletePostMedia = `
	DELETE FROM post_uploads
	WHERE post_id = $1
	`

	ReadMedia = `
	SELECT
	 pu.post_id,              --PostID
  	 pu.upload_id,            --UploadID
 	 pu.position,             --Position
  	 u.media_type,            --media.Row.MediaType
  	 u.storage_key,           --StorageKey
 	 u.variants->>'thumb',    --ThumbKey
  	 u.blurhash,              --Blurhash
  	 u.width,                 --Width
  	 u.height                 --Height
	FROM post_uploads pu
	JOIN uploads u ON u.id = pu.upload_id AND u.status = 'ready'
	WHERE pu.post_id = ANY($1::uuid[])
	ORDER BY pu.post_id, pu.position
	`

	// $1 post_id
	SelectPostByID = `
	SELECT
		p.id,
		p.author_id,
		u.username AS author_username,
		av.media_type AS author_avatar_type,
		av.storage_key AS author_avatar_key,
		av.variants->>'thumb' AS author_avatar_thumb,
		av.blurhash AS author_avatar_blurhash,
		p.title,
		p.body,
		p.created_at,
		p.updated_at,
		COALESCE(u.is_blocked, FALSE) AS is_blocked
	FROM posts p
	JOIN users u ON u.id = p.author_id
	LEFT JOIN uploads av ON av.id = u.avatar_upload_id AND av.status = 'ready' AND av.bucket = 'public'
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
		av.media_type AS author_avatar_type,
		av.storage_key AS author_avatar_key,
		av.variants->>'thumb' AS author_avatar_thumb,
		av.blurhash AS author_avatar_blurhash,
		p.title,
		p.body,
		p.created_at,
		p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id
	LEFT JOIN uploads av ON av.id = u.avatar_upload_id AND av.status = 'ready' AND av.bucket = 'public'
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
		av.media_type AS author_avatar_type,
		av.storage_key AS author_avatar_key,
		av.variants->>'thumb' AS author_avatar_thumb,
		av.blurhash AS author_avatar_blurhash,
		p.title,
		p.body,
		p.created_at,
		p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id
	LEFT JOIN uploads av ON av.id = u.avatar_upload_id AND av.status = 'ready' AND av.bucket = 'public'
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
