package repository

import (
	"context"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BaseRepository interface {
	InitMigration(ctx context.Context) error
}

type BaseRepositoryImpl struct {
	Database *pgxpool.Pool
}

func NewBaseRepositoryImpl(db *pgxpool.Pool) *BaseRepositoryImpl {
	return &BaseRepositoryImpl{Database: db}
}

func (b *BaseRepositoryImpl) InitMigration(ctx context.Context) error {
	sqlBytes, err := os.ReadFile("sql/import.sql")
	if err != nil {
		return err
	}
	sqlText := string(sqlBytes)
	statements := strings.Split(sqlText, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := b.Database.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
