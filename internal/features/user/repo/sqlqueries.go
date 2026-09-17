package repo

const (
	SelectHashedPassword = `
	SELECT hashed_password FROM users WHERE user_id = $1
	`
	UpdateCurrentPassword = `
	UPDATE users 
	SET hashed_password = $1
	WHERE user_id = $2
	`

	CheckIfAbleToChangeReferrer = `
	
	`
)
