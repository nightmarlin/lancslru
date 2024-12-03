package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nightmarlin/lancslru/cmd/birthdayserver/internal"
)

type BirthdayGetter func(context.Context, internal.Name) (internal.Birthday, error)

func GetBirthdayHandler(getBirthday BirthdayGetter) (string, http.Handler) {
	const namePathValue = `name`
	return fmt.Sprintf("GET /{%s}", namePathValue),
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				n := r.PathValue(namePathValue)
				if n == "" {
					err := fmt.Errorf("%s is required", namePathValue)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				date, err := getBirthday(ctx, internal.Name(n))
				if err != nil {
					if errors.Is(err, internal.ErrNotFound) {
						http.NotFound(w, r)
						return
					}

					slog.ErrorContext(
						ctx,
						"failed to fetch birthday",
						slog.String("name", n),
						slog.String("error", err.Error()),
					)

					err = fmt.Errorf("an unknown error occurred: %w", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				_, _ = fmt.Fprintf(w, `{"birthday":"%s"}`, date)
			},
		)
}
