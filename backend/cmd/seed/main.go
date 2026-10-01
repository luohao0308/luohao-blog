// Command seed brings reference data into an environment: the author account
// (registration is closed, so this is the only supported way to create
// accounts) and the demo articles that give a fresh deployment content.
//
// Usage (from backend/):
//
//	go run ./cmd/seed -conf ./configs -email author@example.com -password 'longenough1' -name '罗豪'
//	go run ./cmd/seed -conf ./configs -demo-articles
//
// Both modes can run in one invocation. The same config file as the server is
// loaded, so the database DSN comes from config.yaml (DATABASE_SOURCE env
// override applies). Pending schema migrations are applied before seeding,
// which keeps the command safe to run on a fresh database.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
)

var (
	flagConf        string
	flagEmail       string
	flagPassword    string
	flagName        string
	flagRole        string
	flagDemoArticle bool
	flagReset       bool
)

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	flag.StringVar(&flagEmail, "email", "", "login email of the account (required unless -demo-articles)")
	flag.StringVar(&flagPassword, "password", "", "plaintext password, at least 8 characters")
	flag.StringVar(&flagName, "name", "", "display name shown on the site")
	flag.StringVar(&flagRole, "role", "admin", "account role: admin (default) or reader")
	flag.BoolVar(&flagDemoArticle, "demo-articles", false, "seed the demo articles bundled with the command")
	flag.BoolVar(&flagReset, "reset", false, "with -demo-articles: overwrite existing demo articles with the bundled content")
}

func main() {
	flag.Parse()
	wantAccount := flagEmail != "" || flagPassword != "" || flagName != ""
	if !wantAccount && !flagDemoArticle {
		fmt.Fprintln(os.Stderr, "seed: pass -demo-articles and/or -email/-password")
		flag.Usage()
		os.Exit(2)
	}
	var role biz.UserRole
	switch flagRole {
	case "admin":
		role = biz.UserRoleAdmin
	case "reader":
		role = biz.UserRoleReader
	default:
		fmt.Fprintf(os.Stderr, "seed: unknown -role %q (want admin or reader)\n", flagRole)
		os.Exit(2)
	}

	c := config.New(
		config.WithSource(
			file.NewSource(flagConf),
			env.NewSource("KRATOS"),
		),
	)
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: load config: %v\n", err)
		os.Exit(1)
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		fmt.Fprintf(os.Stderr, "seed: scan config: %v\n", err)
		os.Exit(1)
	}

	// Redis is deliberately not wired here: seeding only touches MySQL.
	store, cleanup, err := data.NewData(bc.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: open data: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	ctx := context.Background()
	if flagDemoArticle {
		articles := biz.NewArticleUsecase(data.NewArticleRepo(store, nil), nil)
		if err := seedDemoArticles(ctx, articles, flagReset); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if !wantAccount {
		return
	}
	users := biz.NewUserUsecase(data.NewUserRepo(store))
	u, err := users.CreateAccount(
		ctx,
		flagEmail,
		flagPassword,
		flagName,
		role,
	)
	if err != nil {
		switch {
		case biz.ErrUserEmailConflict.Is(err):
			fmt.Fprintf(os.Stderr, "seed: email %s is already registered\n", flagEmail)
		case biz.ErrUserInvalidArgument.Is(err):
			fmt.Fprintf(os.Stderr, "seed: invalid email or password shorter than 8 characters\n")
		default:
			fmt.Fprintf(os.Stderr, "seed: create account: %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Printf("seed: created %s %s <%s>\n", flagRole, u.DisplayName, u.Email)
}
