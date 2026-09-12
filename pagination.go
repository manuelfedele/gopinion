package gopinion

import (
	"encoding/json"
	"errors"
	"fmt"
)

// PageRequest contains validated pagination bounds.
type PageRequest struct {
	Limit  int
	Offset int
}

// Page is the only collection response accepted by a list route.
type Page[T any] struct {
	items   []T
	total   int64
	request PageRequest
}

// NewPage creates a validated paginated response.
func NewPage[T any](items []T, total int64, request PageRequest) (Page[T], error) {
	if request.Limit <= 0 {
		return Page[T]{}, errors.New("limit must be greater than zero")
	}
	if request.Offset < 0 {
		return Page[T]{}, errors.New("offset must not be negative")
	}
	if total < 0 {
		return Page[T]{}, errors.New("total items must not be negative")
	}
	if len(items) > request.Limit {
		return Page[T]{}, fmt.Errorf("page contains %d items but limit is %d", len(items), request.Limit)
	}
	if int64(len(items)) > total {
		return Page[T]{}, fmt.Errorf("page contains %d items but total is %d", len(items), total)
	}
	offset := int64(request.Offset)
	if len(items) > 0 && (offset >= total || int64(len(items)) > total-offset) {
		return Page[T]{}, fmt.Errorf("page items exceed total at offset %d", offset)
	}

	copiedItems := make([]T, len(items))
	copy(copiedItems, items)
	return Page[T]{
		items:   copiedItems,
		total:   total,
		request: request,
	}, nil
}

// Items returns a copy of the page's items.
func (page Page[T]) Items() []T {
	items := make([]T, len(page.items))
	copy(items, page.items)
	return items
}

// TotalItems returns the total number of items across all pages.
func (page Page[T]) TotalItems() int64 {
	return page.total
}

func (page Page[T]) pageMarker() {}

func (page Page[T]) pagination() (PageRequest, int64) {
	return page.request, page.total
}

func (page Page[T]) validate() error {
	if page.request.Limit <= 0 || page.request.Offset < 0 {
		return errors.New("page was not created with gopinion.NewPage")
	}
	offset := int64(page.request.Offset)
	if page.total < 0 || len(page.items) > page.request.Limit || int64(len(page.items)) > page.total ||
		len(page.items) > 0 && (offset >= page.total || int64(len(page.items)) > page.total-offset) {
		return errors.New("page is invalid")
	}
	return nil
}

func (page Page[T]) validateRequest(request PageRequest) error {
	if page.request != request {
		return errors.New("page request does not match the validated request")
	}
	return page.validate()
}

// MarshalJSON emits the framework's fixed collection envelope.
func (page Page[T]) MarshalJSON() ([]byte, error) {
	if err := page.validate(); err != nil {
		return nil, err
	}

	return json.Marshal(struct {
		Data       []T `json:"data"`
		Pagination struct {
			Limit      int   `json:"limit"`
			Offset     int   `json:"offset"`
			TotalItems int64 `json:"totalItems"`
		} `json:"pagination"`
	}{
		Data: page.items,
		Pagination: struct {
			Limit      int   `json:"limit"`
			Offset     int   `json:"offset"`
			TotalItems int64 `json:"totalItems"`
		}{
			Limit:      page.request.Limit,
			Offset:     page.request.Offset,
			TotalItems: page.total,
		},
	})
}
