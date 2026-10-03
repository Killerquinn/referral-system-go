package jwtcontext

import "context"

type contextKey string

const UserIDKey contextKey = "user_id"

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}
