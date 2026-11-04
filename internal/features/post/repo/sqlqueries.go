package repo

// TODO: write the queries. Placeholders below only document the expected params.
const (
	// $1 author_id, $2 title, $3 body
	InsertPost = ``

	// $1 post_id
	SelectPostByID = ``

	// $1 post_id, $2 author_id, $3 title (nullable), $4 body (nullable)
	UpdatePostByIDAndAuthor = ``

	// $1 post_id, $2 author_id
	DeletePostByIDAndAuthor = ``

	// $1 author_id, $2 cursor created_at (nullable), $3 cursor id (nullable), $4 limit
	SelectPostsByAuthorID = ``

	// $1 author username, $2 cursor created_at (nullable), $3 cursor id (nullable), $4 limit
	SelectPostsByAuthorUsername = ``

	// $1 reader_id, $2 author username — true if reader is the author or author's referral
	CanReadPostsOf = ``
)
