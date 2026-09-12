package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/manuelfedele/gopinion"
)

type todo struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	OwnerID string `json:"-"`
}

var todos = []todo{
	{ID: 1, Title: "Define opinions", OwnerID: "example-user"},
	{ID: 2, Title: "Enforce authentication", OwnerID: "example-user"},
	{ID: 3, Title: "Enforce authorization", OwnerID: "another-user"},
	{ID: 4, Title: "Generate contracts", OwnerID: "example-user"},
}

type todoScope struct {
	OwnerID string
}

type exampleAuthenticator struct {
	tokenHash [sha256.Size]byte
}

func (authenticator exampleAuthenticator) Authenticate(request *http.Request) (gopinion.Principal, error) {
	presented, found := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
	presentedHash := sha256.Sum256([]byte(presented))
	if !found || subtle.ConstantTimeCompare(presentedHash[:], authenticator.tokenHash[:]) != 1 {
		return gopinion.Principal{}, gopinion.ErrUnauthenticated
	}
	return gopinion.Principal{Subject: "example-user"}, nil
}

type exampleAuthorizer struct{}

func (exampleAuthorizer) Authorize(_ context.Context, principal gopinion.Principal, request gopinion.AuthorizationRequest) (gopinion.AuthorizationDecision, error) {
	switch request.Action {
	case "todo:list":
		if principal.Subject == "" {
			return gopinion.Deny, nil
		}
		return gopinion.Allow, nil
	case "todo:read":
		ownerID, ok := request.Resource.Attributes["owner_id"].(string)
		if !ok || ownerID == "" {
			return gopinion.Deny, errors.New("todo owner is missing")
		}
		if ownerID == principal.Subject {
			return gopinion.Allow, nil
		}
		return gopinion.Deny, nil
	default:
		return gopinion.Deny, nil
	}
}

func main() {
	token := os.Getenv("GOPINION_EXAMPLE_TOKEN")
	if token == "" {
		log.Fatal("GOPINION_EXAMPLE_TOKEN must be set")
	}

	app, err := gopinion.New(
		gopinion.WithAuthenticator(exampleAuthenticator{tokenHash: sha256.Sum256([]byte(token))}),
		gopinion.WithAuthorizer(exampleAuthorizer{}),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Register(gopinion.AuthorizedList("/todos", prepareListTodos, listTodos)); err != nil {
		log.Fatal(err)
	}
	if err := app.Register(gopinion.AuthorizedGet("/todos/{id}", prepareGetTodo, getTodo)); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func prepareListTodos(ctx gopinion.Context, _ gopinion.PageRequest) (gopinion.AuthorizationPlan[todoScope], error) {
	return gopinion.AuthorizationPlan[todoScope]{
		Request: gopinion.AuthorizationRequest{
			Action:   "todo:list",
			Resource: gopinion.AuthorizationResource{Type: "todo_collection", ID: "todos"},
		},
		Value: todoScope{OwnerID: ctx.Principal().Subject},
	}, nil
}

func listTodos(_ gopinion.Context, request gopinion.PageRequest, scope todoScope) (gopinion.Page[todo], error) {
	visible := make([]todo, 0, len(todos))
	for _, candidate := range todos {
		if candidate.OwnerID == scope.OwnerID {
			visible = append(visible, candidate)
		}
	}
	start := request.Offset
	if start > len(visible) {
		start = len(visible)
	}
	end := start + request.Limit
	if end > len(visible) {
		end = len(visible)
	}
	return gopinion.NewPage(visible[start:end], int64(len(visible)), request)
}

func prepareGetTodo(ctx gopinion.Context) (gopinion.AuthorizationPlan[todo], error) {
	id, err := strconv.Atoi(ctx.PathValue("id"))
	if err != nil || id <= 0 {
		return gopinion.AuthorizationPlan[todo]{}, gopinion.NewHTTPError(http.StatusBadRequest, "invalid_id", "The todo ID must be a positive integer.")
	}
	for _, candidate := range todos {
		if candidate.ID == id {
			return gopinion.AuthorizationPlan[todo]{
				Request: gopinion.AuthorizationRequest{
					Action: "todo:read",
					Resource: gopinion.AuthorizationResource{
						Type:       "todo",
						ID:         fmt.Sprint(candidate.ID),
						Attributes: map[string]any{"owner_id": candidate.OwnerID},
					},
				},
				Value: candidate,
			}, nil
		}
	}
	return gopinion.AuthorizationPlan[todo]{}, gopinion.NewHTTPError(http.StatusNotFound, "not_found", "The todo does not exist.")
}

func getTodo(_ gopinion.Context, candidate todo) (todo, error) {
	return candidate, nil
}
