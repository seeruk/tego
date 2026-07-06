package catalog

import (
	"context"
	"errors"
	"strings"
)

var ErrBookNotFound = errors.New("book not found")

type Catalog struct {
	UnimplementedCatalogService
}

func (Catalog) GetBook(ctx context.Context, id string) (Book, error) {
	id = strings.TrimSpace(id)
	if id != "tego" {
		return Book{}, ErrBookNotFound
	}
	return Book{
		ID:     contextBookID(ctx),
		Title:  "Tego: Boundaries in Plain Go",
		Author: "Seer UK",
	}, nil
}

type bookIDContextKey struct{}

func contextWithBookID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, bookIDContextKey{}, id)
}

func contextBookID(ctx context.Context) string {
	if id, ok := ctx.Value(bookIDContextKey{}).(string); ok {
		return id
	}
	return ""
}
