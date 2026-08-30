package repopackage

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
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

func (db *Repository) UserExists(ctx context.Context, email string) (bool, error) {
	const op = "user/repo.UserExist"

	conn, err := db.GetConn()
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
