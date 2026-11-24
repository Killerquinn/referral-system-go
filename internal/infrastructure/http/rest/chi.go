package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/killerquinn/referral-system-go/internal/config"
	authhttphandler "github.com/killerquinn/referral-system-go/internal/features/auth/http-handler"
	httphandler "github.com/killerquinn/referral-system-go/internal/features/auth/http-handler"
	authservice "github.com/killerquinn/referral-system-go/internal/features/auth/service"
	pagehttphandler "github.com/killerquinn/referral-system-go/internal/features/page/http"
	pageservice "github.com/killerquinn/referral-system-go/internal/features/page/service"
	posthttphandler "github.com/killerquinn/referral-system-go/internal/features/post/http"
	postservice "github.com/killerquinn/referral-system-go/internal/features/post/service"
	referralhttphandler "github.com/killerquinn/referral-system-go/internal/features/referral/http"
	referralservice "github.com/killerquinn/referral-system-go/internal/features/referral/service"
	uploadhttphandler "github.com/killerquinn/referral-system-go/internal/features/upload/http"
	uploadservice "github.com/killerquinn/referral-system-go/internal/features/upload/service"
	userhttphandler "github.com/killerquinn/referral-system-go/internal/features/user/http"
	userservice "github.com/killerquinn/referral-system-go/internal/features/user/service"
	localmdw "github.com/killerquinn/referral-system-go/internal/infrastructure/http/middleware"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type App struct {
	server *http.Server
	router *chi.Mux
	port   string
}

func NewApp(log *zap.Logger, cfg *config.Config, auth *authservice.Auth, user *userservice.Uservice, referral *referralservice.RefService, ps *postservice.PostService, us *uploadservice.UploadService, pgs *pageservice.PageService) *App {
	r := chi.NewRouter()

	log.Info("adding middleware")

	//middlewares

	r.Use(middleware.RequestID)

	r.Use(middleware.Logger)

	r.Use(middleware.Recoverer)

	// replaces deprecated middleware.RealIP (trusted spoofable X-Forwarded-For / X-Real-IP)
	r.Use(localmdw.ClientIPResolver(cfg.Server.TrustedProxies))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.Server.CORSOrigins, // config: server.cors_origins / APP_CORS_ORIGINS
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// liveness probe for Docker / the reverse proxy: the process is up and serving HTTP.
	// Deliberately does not touch the DB, so a DB blip doesn't get the container restarted.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("yep, im still alive ;P "))
	})

	// separate limiters so IP and email keys never share one map
	loginIPLimiter := localmdw.NewRateLimiter(rate.Every(12*time.Second), 5, 10*time.Minute)
	loginEmailLimiter := localmdw.NewRateLimiter(rate.Every(12*time.Second), 5, 10*time.Minute)

	newAuthHandler := httphandler.NewAuthHandler(auth, loginEmailLimiter)
	newUserHandler := userhttphandler.NewUserHandler(user)
	newReferralHandler := referralhttphandler.NewRefferalHandler(referral)
	newPostHandler := posthttphandler.NewPostHandler(ps)
	newUploadHandler := uploadhttphandler.NewUploadHandler(us)
	newPageHandler := pagehttphandler.NewPageHandler(pgs)
	viewLimiter := localmdw.NewRateLimiter(rate.Every(3*time.Second), 20, 10*time.Minute)

	// public: token optional, never 401 (anonymous visitors see published pages)
	r.Group(func(r chi.Router) {
		r.Use(localmdw.OptionalAuthMiddleware(cfg.JWT.Secret))
		r.Get("/api/v1/pages/{slug}", newPageHandler.GetPublicPage)
		// dedupe makes repeats harmless; the limit stops one IP from inflating views by faking user agents
		r.With(viewLimiter.ByIP).Post("/api/v1/pages/{slug}/view", newPageHandler.RecordView)
	})

	r.Group(func(r chi.Router) {
		r.Use(localmdw.AuthMiddleware(cfg.JWT.Secret))
		//auth
		r.Post("/auth/logout-all", newAuthHandler.UserLogOutAll)
		// email verification: the DB cooldown (60s) and the 5-attempt lock already limit these
		r.Post("/auth/verify-email/send", newAuthHandler.SendEmailVerification)
		r.Post("/auth/verify-email", newAuthHandler.VerifyEmail)
		//user
		r.Put("/user/changepassword", newUserHandler.ChangeUserPassword)
		r.Put("/user/changereferrer", newUserHandler.ChangeReferrer)
		r.Post("/user/createkey", newUserHandler.CreateUniqueReferralKey)
		r.Put("/user/avatar", newUserHandler.SetAvatar)

		//===================================================================
		//getuser
		r.Get("/user/{identifier}", newUserHandler.UserByUsername)
		//getself
		r.Get("/user", newUserHandler.UserByUsername)
		//===================================================================

		r.Post("/user/deleteaccount", newUserHandler.DeleteUsersAccount)
		//referral
		r.Get("/api/v1/referrals/{identifier}", newReferralHandler.CurrentReferralList)
		r.Get("/api/v1/referrals", newReferralHandler.CurrentReferralList)
		r.Post("/api/v1/referrals/draw", newReferralHandler.ContestBetweenReferrals)
		r.Get("/api/v1/referralinfo/{identifier}", newReferralHandler.SeeWhoseReferralAlready)
		r.Get("/api/v1/referral-stats", newReferralHandler.ReferralStats)
		//reward
		//post
		r.Post("/api/v1/posts", newPostHandler.CreatePost)
		r.Get("/api/v1/posts/feed", newPostHandler.Feed)
		r.Get("/api/v1/posts/user", newPostHandler.PostsOfUser)              // own posts
		r.Get("/api/v1/posts/user/{identifier}", newPostHandler.PostsOfUser) // posts of a user
		r.Get("/api/v1/posts/{postID}", newPostHandler.GetPost)
		r.Put("/api/v1/posts/{postID}", newPostHandler.UpdatePost)
		r.Delete("/api/v1/posts/{postID}", newPostHandler.DeletePost)
		//upload (static "quota" wins over {uploadID} in chi)
		r.Post("/api/v1/uploads", newUploadHandler.InitUpload)
		r.Get("/api/v1/uploads/quota", newUploadHandler.GetQuota)
		r.Post("/api/v1/uploads/{uploadID}/complete", newUploadHandler.CompleteUpload)
		r.Get("/api/v1/uploads/{uploadID}", newUploadHandler.GetUpload)
		r.Delete("/api/v1/uploads/{uploadID}", newUploadHandler.DeleteUpload)
		//page (static "me" / "slug-available" win over {slug}; both are reserved slugs too)
		r.Get("/api/v1/pages/me", newPageHandler.GetMyPage)
		r.Post("/api/v1/pages", newPageHandler.CreatePage)
		r.Put("/api/v1/pages/me", newPageHandler.UpdateMyPage)
		r.Put("/api/v1/pages/me/slug", newPageHandler.ChangeSlug)
		r.Get("/api/v1/pages/slug-available", newPageHandler.CheckSlug) // ?slug=
	})

	log.Info("setup server")
	port := strconv.Itoa(cfg.Server.Port)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           r,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
	}

	authhttphandler.Register(r, auth, loginIPLimiter, loginEmailLimiter)
	userhttphandler.Register(r, user)
	referralhttphandler.Register(r, referral)

	return &App{
		server: server,
		router: r,
		port:   port,
	}
}

func (a *App) Run(logger *zap.Logger) error {
	const op = "Run"

	logger.Info("starting http server",
		zap.String("op", op),
		zap.String("addr", a.server.Addr),
	)

	if err := a.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("server stopped gracefully", zap.String("op", op))
			return nil
		}
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (a *App) Shutdown(logger *zap.Logger, ctx context.Context) error {
	const op = "Shutdown"
	logger.Info(op)
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown: %w", err)
	}
	return nil
}
