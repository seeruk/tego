package books

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrBookNotFound = errors.New("book not found")

type InvalidBookIDError struct {
	ID string
}

func (e InvalidBookIDError) Error() string {
	return fmt.Sprintf("invalid book ID %q", e.ID)
}

type BookStore struct {
	UnimplementedBookService
}

func (BookStore) GetBook(ctx context.Context, id string) (Book, error) {
	if strings.TrimSpace(id) == "" {
		return Book{}, InvalidBookIDError{ID: id}
	}
	if id != "tego" {
		return Book{}, ErrBookNotFound
	}
	return Book{
		ID:     "tego",
		Title:  "Tego: Boundaries in Plain Go",
		Author: "Seer UK",
	}, nil
}
