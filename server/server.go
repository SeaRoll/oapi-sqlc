package server

import (
	"context"
	"fmt"

	"github.com/SeaRoll/oapi-sqlc/api"
	"github.com/SeaRoll/oapi-sqlc/database"
	"github.com/go-playground/validator/v10"
	"github.com/oapi-codegen/runtime/types"
)

// ensure that we've conformed to the `StrictServerInterface` with a compile-time check.
var (
	_ api.StrictServerInterface = (*Server)(nil)
	v                           = validator.New()
)

type Server struct {
	db database.Database
}

// PostApiV1Books implements api.StrictServerInterface.
func (s Server) PostApiV1Books(ctx context.Context, request api.PostApiV1BooksRequestObject) (api.PostApiV1BooksResponseObject, error) {
	err := v.Struct(request.Body)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var book database.Book
	// create a new book
	err = s.db.WithTX(ctx, func(q database.Querier) error {
		var err error

		book, err = q.CreateBook(ctx, database.CreateBookParams{
			Title:         request.Body.Title,
			Author:        request.Body.Author,
			PublishedDate: request.Body.PublishedDate.Time,
		})
		if err != nil {
			return fmt.Errorf("failed to create book: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create book: %w", err)
	}

	return api.PostApiV1Books201JSONResponse(api.BookDTO{
		Id:            int(book.ID),
		Author:        book.Author,
		PublishedDate: types.Date{Time: book.PublishedDate},
		Title:         book.Title,
	}), nil
}

func NewServer(db database.Database) Server {
	return Server{db: db}
}
