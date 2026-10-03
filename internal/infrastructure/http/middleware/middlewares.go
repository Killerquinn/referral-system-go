package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
)

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "ERR_1_NO_HEADER", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})

			if err != nil {
				log.Printf("--> [JWT PARSE ERROR]: %v", err)
				http.Error(w, "ERR_2_INVALID_JWT", http.StatusUnauthorized)
				return
			}

			if !token.Valid {
				http.Error(w, "ERR_3_TOKEN_NOT_VALID", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "ERR_4_BAD_CLAIMS_TYPE", http.StatusUnauthorized)
				return
			}

			subStr, ok := claims["sub"].(string)
			if !ok {
				log.Printf("--> [CLAIMS SUB ERROR]: claims['sub'] is %T (%v)", claims["sub"], claims["sub"])
				http.Error(w, "ERR_5_SUB_NOT_STRING", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), jwtcontext.UserIDKey, subStr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
