package repo

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (db *Repository) GetConn() (*pgxpool.Conn, error) {
	const op = "user/repo.GetConn"

	conn, err := db.db.Acquire(context.Background())
	if err != nil {
		log.Panicf("%s - %s", op, err)
	}

	return conn, err
}

func (r *Repository) PullOldPassword(ctx context.Context, userID uuid.UUID) (oldPass []byte, err error) {
	const op = "/internal/features/user/repo.PullOldPassword"

	conn, err := r.GetConn()
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	var oldpassword string

	if err := conn.QueryRow(ctx, SelectHashedPassword, userID).Scan(&oldpassword); err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return []byte(oldpassword), nil

}

func (r *Repository) ChangePassword(ctx context.Context, userID uuid.UUID, newPassword []byte) (err error) {
	const op = "/internal/features/user/repo.ChangePassword"

	conn, err := r.GetConn()
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	result, err := conn.Exec(ctx, UpdateCurrentPassword, string(newPassword), userID)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	affectedRows := result.RowsAffected()
	if affectedRows == 0 {
		return sharederrors.ErrUnnableToChangePassword
	}

	return nil
}
