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

	chimdw "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	"golang.org/x/time/rate"
)

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httpfeatures.WriteError(w, http.StatusUnauthorized, "ERR_1_NO_HEADER")
				return
			}

			tokenStr, found := strings.CutPrefix(authHeader, "Bearer ")
			if !found {
				httpfeatures.WriteError(w, http.StatusUnauthorized, "ERR_1_NO_HEADER")
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
				httpfeatures.WriteError(w, http.StatusUnauthorized, "ERR_2_INVALID_JWT")
				return
			}

			if !token.Valid {
				httpfeatures.WriteError(w, http.StatusUnauthorized, "ERR_3_TOKEN_NOT_VALID")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				httpfeatures.WriteError(w, http.StatusUnauthorized, "ERR_4_BAD_CLAIMS_TYPE")
				return
			}

			subStr, ok := claims["sub"].(string)
			if !ok {
				log.Printf("--> [CLAIMS SUB ERROR]: claims['sub'] is %T (%v)", claims["sub"], claims["sub"])
				httpfeatures.WriteError(w, http.StatusUnauthorized, "ERR_5_SUB_NOT_STRING")
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
		if !rl.Allow(ClientIP(r)) {
			w.Header().Set("Retry-After", rl.retryAfter())
			httpfeatures.WriteError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIPResolver picks how the client IP is resolved and stores it in the request context.
// No trusted proxies: IP of the TCP peer. Otherwise X-Forwarded-For is walked right-to-left,
// skipping the trusted proxy ranges, so a client can't spoof its IP with headers.
// Replaces chi's deprecated RealIP, which trusted X-Forwarded-For/X-Real-IP from anyone.
func ClientIPResolver(trustedProxies []string) func(http.Handler) http.Handler {
	if len(trustedProxies) == 0 {
		return chimdw.ClientIPFromRemoteAddr
	}
	return chimdw.ClientIPFromXFF(trustedProxies...)
}

// ClientIP returns the IP resolved by ClientIPResolver. If it couldn't be resolved
// (e.g. malformed X-Forwarded-For) it falls back to the TCP peer address.
func ClientIP(r *http.Request) string {
	if ip := chimdw.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
