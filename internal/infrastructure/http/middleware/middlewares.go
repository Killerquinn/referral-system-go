package middleware

import (
	"context"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	"golang.org/x/time/rate"
)

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "ERR_1_NO_HEADER", http.StatusUnauthorized)
				return
			}

			tokenStr, found := strings.CutPrefix(authHeader, "Bearer ")
			if !found {
				http.Error(w, "ERR_1_NO_HEADER", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			},
				jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
				jwt.WithExpirationRequired(),
			)

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

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*client

	limit rate.Limit
	burst int
	ttl   time.Duration
}

func NewRateLimiter(limit rate.Limit, burst int, ttl time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*client),
		limit:   limit,
		burst:   burst,
		ttl:     ttl,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	c, ok := rl.clients[key]
	if !ok {
		c = &client{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.clients[key] = c
	}
	c.lastSeen = time.Now()

	return c.limiter.Allow()
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for key, c := range rl.clients {
			if time.Since(c.lastSeen) > rl.ttl {
				delete(rl.clients, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) retryAfter() string {
	seconds := math.Ceil(1 / float64(rl.limit))
	return strconv.Itoa(int(seconds))
}

func (rl *RateLimiter) ByIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.Allow(ip) {
			w.Header().Set("Retry-After", rl.retryAfter())
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
