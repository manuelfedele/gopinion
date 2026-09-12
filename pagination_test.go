package gopinion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewPageCopiesItems(t *testing.T) {
	items := []string{"one"}
	page, err := NewPage(items, 1, PageRequest{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	items[0] = "changed"
	returned := page.Items()
	returned[0] = "also changed"
	if page.Items()[0] != "one" {
		t.Fatal("Page did not retain an isolated item slice")
	}
}

func TestNewPageRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		total   int64
		request PageRequest
	}{
		{name: "zero limit", total: 0, request: PageRequest{Limit: 0}},
		{name: "negative offset", total: 0, request: PageRequest{Limit: 10, Offset: -1}},
		{name: "negative total", total: -1, request: PageRequest{Limit: 10}},
		{name: "too many page items", items: []string{"one", "two"}, total: 2, request: PageRequest{Limit: 1}},
		{name: "more items than total", items: []string{"one"}, total: 0, request: PageRequest{Limit: 10}},
		{name: "items beyond total", items: []string{"one"}, total: 3, request: PageRequest{Limit: 10, Offset: 3}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewPage(test.items, test.total, test.request); err == nil {
				t.Fatal("NewPage() error = nil")
			}
		})
	}
}

func TestZeroPageCannotBeMarshaled(t *testing.T) {
	_, err := Page[string]{}.MarshalJSON()
	if err == nil || !strings.Contains(err.Error(), "NewPage") {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
}

func TestEmptyPageMarshalsDataAsArray(t *testing.T) {
	page, err := NewPage([]string{}, 0, PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	payload, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(payload), `"data":[]`) {
		t.Fatalf("payload = %s, want empty data array", payload)
	}
	if !strings.Contains(string(payload), `"pagination":{"limit":10,"offset":0,"totalItems":0}`) {
		t.Fatalf("payload = %s, want camelCase pagination metadata", payload)
	}
}
