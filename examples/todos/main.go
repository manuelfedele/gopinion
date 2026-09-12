package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
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
	ID    int    `json:"id"`
	Title string `json:"title"`
}

var todos = []todo{
	{ID: 1, Title: "Define opinions"},
	{ID: 2, Title: "Enforce authentication"},
	{ID: 3, Title: "Enforce pagination"},
	{ID: 4, Title: "Generate contracts"},
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

func main() {
	token := os.Getenv("GOPINION_EXAMPLE_TOKEN")
	if token == "" {
		log.Fatal("GOPINION_EXAMPLE_TOKEN must be set")
	}

	app, err := gopinion.New(
		"gopinion.yaml",
		gopinion.WithAuthenticator(exampleAuthenticator{tokenHash: sha256.Sum256([]byte(token))}),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Register(gopinion.List("/todos", listTodos)); err != nil {
		log.Fatal(err)
	}
	if err := app.Register(gopinion.Get("/todos/{id}", getTodo)); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func listTodos(_ gopinion.Context, request gopinion.PageRequest) (gopinion.Page[todo], error) {
	start := request.Offset()
	if start > len(todos) {
		start = len(todos)
	}
	end := start + request.Size
	if end > len(todos) {
		end = len(todos)
	}
	return gopinion.NewPage(todos[start:end], int64(len(todos)), request)
}

func getTodo(ctx gopinion.Context) (todo, error) {
	id, err := strconv.Atoi(ctx.PathValue("id"))
	if err != nil || id <= 0 {
		return todo{}, gopinion.NewHTTPError(http.StatusBadRequest, "invalid_id", "The todo ID must be a positive integer.")
	}
	for _, candidate := range todos {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return todo{}, gopinion.NewHTTPError(http.StatusNotFound, "not_found", "The todo does not exist.")
}
