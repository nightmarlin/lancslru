package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/nightmarlin/lancslru/cmd/birthdayserver/internal"
)

type DB struct {
	conn *pgx.Conn
}

func New(ctx context.Context, conn *pgx.Conn) (*DB, error) {
	if _, err := conn.Exec(
		ctx,
		`create table if not exists birthdays (name text primary key not null, date date not null)`,
	); err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

func (db *DB) InsertBirthdays(ctx context.Context, birthdays map[internal.Name]string) error {
	for name, date := range birthdays {
		if _, err := db.conn.Exec(
			ctx,
			`insert into birthdays ("name", "date") values ($1::text, $2::date) on conflict ("name") do update set date = $2`,
			name.Normalize(),
			date,
		); err != nil {
			return fmt.Errorf("inserting birthday for %q: %w", name, err)
		}
	}
	return nil
}

func (db *DB) LookupBirthday(ctx context.Context, name internal.Name) (internal.Birthday, error) {
	name = name.Normalize()

	var date time.Time
	if err := db.
		conn.
		QueryRow(ctx, `select "date" from birthdays where "name" = $1`, name).
		Scan(&date); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return internal.Birthday{}, internal.ErrNotFound
		}
		return internal.Birthday{}, fmt.Errorf("looking up birthday: %w", err)
	}

	slog.InfoContext(
		ctx,
		"loaded birthday from db",
		slog.String("name", name.String()),
		slog.String("date", date.Format("2006-01-02")),
	)
	return internal.Birthday(date), nil
}
