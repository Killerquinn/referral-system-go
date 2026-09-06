package repopackage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/domain/auth"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(dsn string) *Repository {
	const op = "user/repo.New"

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Panicf("%s - %s", op, err)
	} //add error handling without panic

	if db == nil {
		log.Panicf("%s - %s", op, "db is nil")
	}

	if err := db.Ping(context.Background()); err != nil {
		log.Panicf("%s - %s", op, err)
	}

	return &Repository{
		db: db,
	}
}

func (r *Repository) GetConn() (*pgxpool.Conn, error) {
	const op = "user/repo.GetConn"

	conn, err := r.db.Acquire(context.Background())
	if err != nil {
		log.Panicf("%s - %s", op, err)
	}

	return conn, err
}

func (r *Repository) User(ctx context.Context, email string) (*auth.User, error) {
	const op = "user/repo.User"

	conn, err := r.GetConn()
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	rows, err := conn.Query(ctx, getUserByEmail, email)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	defer rows.Close()

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[auth.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sharederrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return &user, nil
}

func (r *Repository) UserExists(ctx context.Context, email string) (bool, error) {
	const op = "user/repo.UserExist"

	conn, err := r.GetConn()
	if err != nil {
		return true, fmt.Errorf("%s:%w", op, err)
	}
	defer conn.Release()

	var isUserExist bool

	err = conn.QueryRow(ctx, selectIfUserExist).Scan(&isUserExist)
	if err != nil {
		return true, fmt.Errorf("%s:%w", op, err)
	}

	if isUserExist {
		return true, fmt.Errorf("user with that email already exist")
	}

	return false, nil
}

func (r *Repository) SaveNewUser(ctx context.Context, username string, email string, password []byte) (uid string, err error) {
	const op = "user/repo.SaveNewUser"

	conn, err := r.GetConn()
	if err != nil {
		return "", fmt.Errorf("%s:%w", op, err)
	}
	defer conn.Release()

	var id string

	err = conn.QueryRow(ctx, createUserQuery, username, email, password).Scan(&id)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "23505" {
			return "", sharederrors.ErrUserAlreadyRegistered
		}
		return "", fmt.Errorf("%s:failed to create user", op)
	}

	return id, nil

}

func (r *Repository) CreateSession(ctx context.Context, userID uuid.UUID, hashedRefreshToken string, userAgent string, clientIP string, expiresAt time.Time) (refreshToken []byte, err error) {
	const op = "user/repo.LoginUser"

	conn, err := r.GetConn()
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	defer conn.Release()
	panic("")
}
