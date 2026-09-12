package main

import (
	"context"
	"testing"

	"github.com/manuelfedele/gopinion"
)

func TestExampleAuthorizerChecksOwnership(t *testing.T) {
	authorizer := exampleAuthorizer{}
	request := gopinion.AuthorizationRequest{
		Action: "todo:read",
		Resource: gopinion.AuthorizationResource{
			Type:       "todo",
			ID:         "1",
			Attributes: map[string]any{"owner_id": "example-user"},
		},
	}

	decision, err := authorizer.Authorize(context.Background(), gopinion.Principal{Subject: "example-user"}, request)
	if err != nil || decision != gopinion.Allow {
		t.Fatalf("owner decision = %d, error = %v", decision, err)
	}
	decision, err = authorizer.Authorize(context.Background(), gopinion.Principal{Subject: "another-user"}, request)
	if err != nil || decision != gopinion.Deny {
		t.Fatalf("non-owner decision = %d, error = %v", decision, err)
	}
}

func TestListTodosFiltersBeforePagination(t *testing.T) {
	request := gopinion.PageRequest{Page: 1, Size: 10}
	page, err := listTodos(gopinion.Context{}, request, todoScope{OwnerID: "example-user"})
	if err != nil {
		t.Fatalf("listTodos() error = %v", err)
	}
	if page.TotalItems() != 3 || len(page.Items()) != 3 {
		t.Fatalf("page contains %d of %d items, want 3 of 3", len(page.Items()), page.TotalItems())
	}
	for _, item := range page.Items() {
		if item.OwnerID != "example-user" {
			t.Fatalf("unauthorized todo returned: %+v", item)
		}
	}
}
