package repopackage

const (
	selectIfUserExist = `
	SELECT EXISTS(SELECT 1 FROM users WHERE email = '$1')
	`
)
