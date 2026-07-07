package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/seeruk/tego"
	"github.com/seeruk/tego/examples/hooks/catalogpbv1"
)

func ServiceHooks() CatalogServiceHooks {
	var hooks CatalogServiceHooks
	hooks.AddPreGetBookRequestMappingHook(
		// Normalize the raw protobuf request before Tego maps it.
		func(ctx context.Context, info tego.RPCInfo, request *catalogpbv1.GetBookRequest) (context.Context, *catalogpbv1.GetBookRequest, error) {
			id := strings.ToLower(strings.TrimSpace(request.GetId()))
			request.SetId(id)
			return ctx, request, nil
		},
	)
	hooks.AddPostGetBookRequestMappingHook(
		// Capture mapped request data in context before the facade method runs.
		func(ctx context.Context, info tego.RPCInfo, request GetBookRequest) (context.Context, GetBookRequest, error) {
			return contextWithBookID(ctx, request.ID), request, nil
		},
	)
	hooks.AddPreGetBookResponseMappingHook(
		// Fill presentation fields before reusable response finalizers run.
		func(ctx context.Context, info tego.RPCInfo, response GetBookResponse) (GetBookResponse, error) {
			if response.Book.DisplayTitle == "" {
				response.Book.DisplayTitle = response.Book.Title + " by " + response.Book.Author
			}
			return response, nil
		},
	)
	hooks.SetPostGetBookResponseMappingHooks(
		// Populate a protobuf-only compatibility field after Tego mapping.
		func(ctx context.Context, info tego.RPCInfo, response *catalogpbv1.GetBookResponse) (*catalogpbv1.GetBookResponse, error) {
			book := response.GetBook()
			if book == nil {
				return response, InvalidBookResponseError{Reason: "missing protobuf book"}
			}
			if strings.TrimSpace(book.GetCatalogRef()) == "" {
				return response, InvalidBookResponseError{Reason: "missing catalog reference"}
			}
			if book.GetLegacyBookId() == "" {
				book.SetLegacyBookId("legacy:" + book.GetCatalogRef())
			}
			return response, nil
		},
	)
	return hooks
}

func InterfaceHooks() tego.InterfaceHooks {
	var hooks tego.InterfaceHooks
	tego.AddPostRequestMappingHook(&hooks, validate)
	tego.AddPreResponseMappingHook(&hooks, finalize)
	tego.AddPostRequestMappingHook(&hooks, func(ctx context.Context, info tego.RPCInfo, i any) (context.Context, error) {
		// Any hooks can inspect all requests or responses in the service, based on which hook you add.
		fmt.Printf("%T\n", i)
		return ctx, nil
	})
	return hooks
}

type validator interface {
	Validate() error
}

func (r GetBookRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return InvalidBookIDError{ID: strings.TrimSpace(r.ID)}
	}
	return nil
}

type finalizer interface {
	Finalize() error
}

func (r *GetBookResponse) Finalize() error {
	if r.Book.CatalogRef == "" {
		r.Book.CatalogRef = catalogRef(r.Book)
	}
	return nil
}

func validate(ctx context.Context, info tego.RPCInfo, validator validator) (context.Context, error) {
	return ctx, validator.Validate()
}

func finalize(ctx context.Context, info tego.RPCInfo, finalizer finalizer) error {
	return finalizer.Finalize()
}

type InvalidBookIDError struct {
	ID string
}

func (e InvalidBookIDError) Error() string {
	return fmt.Sprintf("invalid book ID %q", e.ID)
}

type InvalidBookResponseError struct {
	Reason string
}

func (e InvalidBookResponseError) Error() string {
	return fmt.Sprintf("invalid book response: %s", e.Reason)
}

func catalogRef(book Book) string {
	return "book:" + slug(book.ID) + ":" + slug(book.Author) + ":" + slug(book.Title)
}

func slug(value string) string {
	var parts []string
	var part strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z':
			part.WriteRune(r)
		case r >= '0' && r <= '9':
			part.WriteRune(r)
		case part.Len() > 0:
			parts = append(parts, part.String())
			part.Reset()
		}
	}
	if part.Len() > 0 {
		parts = append(parts, part.String())
	}
	return strings.Join(parts, "-")
}
