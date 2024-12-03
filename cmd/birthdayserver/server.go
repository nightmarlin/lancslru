package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/nightmarlin/lancslru"
	"github.com/nightmarlin/lancslru/cmd/birthdayserver/internal"
	"github.com/nightmarlin/lancslru/cmd/birthdayserver/internal/handlers"
	"github.com/nightmarlin/lancslru/cmd/birthdayserver/internal/postgres"
)

var initBirthdays = map[internal.Name]string{
	"lewis":   "2002-01-22",
	"noah":    "1999-10-13",
	"finn":    "2000-08-11",
	"sabrina": "2002-06-06",
	"leanne":  "1995-09-19",
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := func() error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		conn, err := pgx.Connect(
			ctx,
			"host=localhost port=5432 user=postgres password=password dbname=postgres TimeZone=UTC",
		)
		if err != nil {
			return fmt.Errorf("connecting to db: %w", err)
		}
		defer func() { _ = conn.Close(context.Background()) }()

		db, err := postgres.New(ctx, conn)
		if err != nil {
			return fmt.Errorf("initializing db: %w", err)
		}

		if err := db.InsertBirthdays(ctx, initBirthdays); err != nil {
			return fmt.Errorf("inserting birthday set: %w", err)
		}

		getBirthday := wrapCache(
			lancslru.New[internal.Name, internal.Birthday](2),
			db.LookupBirthday,
		)

		mux := &http.ServeMux{}
		mux.Handle(handlers.GetBirthdayHandler(getBirthday))

		srv := http.Server{Addr: ":8080", Handler: mux}
		go func() {
			<-ctx.Done()
			grace := 5 * time.Second
			slog.InfoContext(ctx, "shutting down server", slog.Duration("grace_period", grace))

			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(grace))
			defer cancel()
			_ = srv.Shutdown(ctx)
		}()

		slog.InfoContext(ctx, "starting server", slog.String("address", srv.Addr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("unexpected error while serving http: %w", err)
		}
		return nil

	}(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func wrapCache[K comparable, V any](
	cache *lancslru.Cache[K, V],
	load func(context.Context, K) (V, error),
) func(context.Context, K) (V, error) {
	return func(ctx context.Context, k K) (V, error) {
		return cache.Lookup(ctx, k, load)
	}
}
