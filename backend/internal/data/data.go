package data

import (
	stdsql "database/sql"
	stderrors "errors"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/migrations"

	"github.com/go-kratos/kratos/v3/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/wire"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewRedis, NewTokenIssuer, NewSessionRepo, NewRateLimiter, NewCommentRateLimiter, NewRefreshTokenTTL, NewAuthorizer, NewArticleRepo, NewCommentRepo, NewUserRepo)

// runMigrations applies all pending migrations idempotently from the embedded
// migrations.FS. The MySQL DSN must enable multiStatements for
// multi-statement migration files.
func runMigrations(db *stdsql.DB) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !stderrors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// Data holds the long-lived storage clients shared by repos.
type Data struct {
	db *ent.Client
}

// NewData opens the database connection, applies versioned migrations, and
// returns the ent client with a cleanup function.
func NewData(c *conf.Data) (*Data, func(), error) {
	dc := c.GetDatabase()
	db, err := stdsql.Open("mysql", dc.GetSource())
	if err != nil {
		return nil, nil, err
	}
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	drv := entsql.OpenDB(dialect.MySQL, db)
	client := ent.NewClient(ent.Driver(drv))
	if dc.GetDebug() {
		client = client.Debug()
	}
	cleanup := func() {
		log.Info("closing the data resources")
		if err := client.Close(); err != nil {
			log.Error("failed closing the database", "err", err)
		}
	}
	return &Data{db: client}, cleanup, nil
}
