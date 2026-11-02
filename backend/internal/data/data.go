package data

import (
	"context"
	stdsql "database/sql"
	stderrors "errors"
	tc "github.com/luohao0308/luohao-blog/backend/internal/timecontract"

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
var ProviderSet = wire.NewSet(NewOfficialAccountPublisher, NewWechatSyncSettings, NewWechatSyncRepo, NewData, NewRedis, NewTokenIssuer, NewSessionRepo, NewRateLimiter, NewRegisterRateLimiter, NewEmailRateLimiter, NewAccountEmailTokenRepo, NewAccountMailSender, NewAccountEmailSettings, NewCommentRateLimiter, NewCommentModerationPolicy, NewSubscribeRateLimiter, NewRefreshTokenTTL, NewAuthorizer, NewArticleRepo, NewCategoryRepo, NewProjectRepo, NewAboutRepo, NewCollectionRepo, NewCommentRepo, NewSubscriberRepo, NewUserRepo, NewAvatarStore, NewMediaStore, NewEsIndexer, NewChatLLM, NewChatRateLimiter, NewSearchRateLimiter, NewWechatClient, NewWechatBindingStore, NewWechatSafetyCache, NewConfiguredWechatContentSafetyClient, NewMiniCommentLimiter, NewMiniCommentRepo)

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
	return NewDataWithStateDir(c, tc.ResolveStateDir(c.GetDatabase().GetTimeContractStateDir(), ""))
}

// NewDataWithStateDir permits explicit owned control directories for tools and
// fixtures; it never skips the same readiness gate used by the server.
func NewDataWithStateDir(c *conf.Data, stateDir string) (*Data, func(), error) {
	stateDir = tc.ResolveStateDir(c.GetDatabase().GetTimeContractStateDir(), stateDir)
	first, e := tc.ReadState(stateDir)
	if e != nil || first.Status != "ready" {
		return nil, nil, tc.ErrContract
	}
	dc := c.GetDatabase()
	db, err := tc.OpenUTC(context.Background(), dc.GetSource())
	if err != nil {
		return nil, nil, err
	}
	if err := tc.RequireReady(context.Background(), db, stateDir); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	last, e := tc.ReadState(stateDir)
	if e != nil || tc.Hash(first) != tc.Hash(last) || tc.RequireReady(context.Background(), db, stateDir) != nil {
		_ = db.Close()
		return nil, nil, tc.ErrContract
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
