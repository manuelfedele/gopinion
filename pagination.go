package gopinion

import (
	"encoding/json"
	"errors"
	"fmt"
)

// PageRequest contains validated, one-based pagination coordinates.
type PageRequest struct {
	Page int
	Size int
}

// Offset returns the zero-based item offset represented by the request.
func (request PageRequest) Offset() int {
	return (request.Page - 1) * request.Size
}

// Page is the only collection response accepted by a list route.
type Page[T any] struct {
	items   []T
	total   int64
	request PageRequest
}

// NewPage creates a validated paginated response.
func NewPage[T any](items []T, total int64, request PageRequest) (Page[T], error) {
	if request.Page <= 0 {
		return Page[T]{}, errors.New("page must be greater than zero")
	}
	if request.Size <= 0 {
		return Page[T]{}, errors.New("page size must be greater than zero")
	}
	maximumInteger := int(^uint(0) >> 1)
	if request.Page > 1 && request.Page-1 > maximumInteger/request.Size {
		return Page[T]{}, errors.New("page offset is too large")
	}
	if total < 0 {
		return Page[T]{}, errors.New("total items must not be negative")
	}
	if len(items) > request.Size {
		return Page[T]{}, fmt.Errorf("page contains %d items but page size is %d", len(items), request.Size)
	}
	if int64(len(items)) > total {
		return Page[T]{}, fmt.Errorf("page contains %d items but total is %d", len(items), total)
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

func (page Page[T]) validate() error {
	if page.request.Page <= 0 || page.request.Size <= 0 {
		return errors.New("page was not created with gopinion.NewPage")
	}
	if page.total < 0 || len(page.items) > page.request.Size || int64(len(page.items)) > page.total {
		return errors.New("page is invalid")
	}
	return nil
}

// MarshalJSON emits the framework's fixed collection envelope.
func (page Page[T]) MarshalJSON() ([]byte, error) {
	if err := page.validate(); err != nil {
		return nil, err
	}

	totalPages := page.total / int64(page.request.Size)
	if page.total%int64(page.request.Size) != 0 {
		totalPages++
	}

	return json.Marshal(struct {
		Data       []T `json:"data"`
		Pagination struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			TotalItems int64 `json:"total_items"`
			TotalPages int64 `json:"total_pages"`
		} `json:"pagination"`
	}{
		Data: page.items,
		Pagination: struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			TotalItems int64 `json:"total_items"`
			TotalPages int64 `json:"total_pages"`
		}{
			Page:       page.request.Page,
			PageSize:   page.request.Size,
			TotalItems: page.total,
			TotalPages: totalPages,
		},
	})
}
