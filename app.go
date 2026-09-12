package gopinion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ErrPaginationRequired indicates that a singular route attempted to return a
// collection while pagination is globally required.
var ErrPaginationRequired = errors.New("pagination is required for collection responses")

type options struct {
	authenticator Authenticator
	logger        *slog.Logger
}

// Option customizes a framework dependency without changing application
// policy. Policy is read only from gopinion.yaml.
type Option interface {
	apply(*options) error
}

type optionFunc func(*options) error

func (option optionFunc) apply(options *options) error {
	return option(options)
}

// WithAuthenticator supplies the implementation used when authentication is
// required by configuration.
func WithAuthenticator(authenticator Authenticator) Option {
	return optionFunc(func(options *options) error {
		if isNil(authenticator) {
			return errors.New("authenticator must not be nil")
		}
		options.authenticator = authenticator
		return nil
	})
}

// WithLogger supplies the structured logger used for internal failures.
func WithLogger(logger *slog.Logger) Option {
	return optionFunc(func(options *options) error {
		if logger == nil {
			return errors.New("logger must not be nil")
		}
		options.logger = logger
		return nil
	})
}

// App owns the complete application router and HTTP server lifecycle.
type App struct {
	config        config
	authenticator Authenticator
	logger        *slog.Logger
	mux           *http.ServeMux

	mu      sync.Mutex
	started bool
	routes  map[string]struct{}
}

// New loads the application's single policy file and constructs the framework.
func New(configPath string, suppliedOptions ...Option) (*App, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	configuredOptions := options{logger: slog.Default()}
	for _, option := range suppliedOptions {
		if option == nil {
			return nil, errors.New("option must not be nil")
		}
		if err := option.apply(&configuredOptions); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}
	if cfg.Authentication.Mode == policyRequired && isNil(configuredOptions.authenticator) {
		return nil, errors.New("authentication is required but no authenticator was supplied")
	}

	return &App{
		config:        cfg,
		authenticator: configuredOptions.authenticator,
		logger:        configuredOptions.logger,
		mux:           http.NewServeMux(),
		routes:        make(map[string]struct{}),
	}, nil
}

// Register adds a framework route. Registration is closed after Run starts.
func (app *App) Register(route Route) error {
	if route == nil {
		return errors.New("route must not be nil")
	}
	definition := route.gopinionRoute()
	if definition.pattern == "" || definition.pattern[0] != '/' {
		return fmt.Errorf("route pattern %q must start with /", definition.pattern)
	}
	if definition.invoke == nil {
		return errors.New("route handler must not be nil")
	}
	if app.config.Pagination.Mode == policyRequired && definition.responseKind == responseSingular && definition.responseCollection {
		return fmt.Errorf("%s %s: %w", definition.method, definition.pattern, ErrPaginationRequired)
	}

	app.mu.Lock()
	defer app.mu.Unlock()
	if app.started {
		return errors.New("routes cannot be registered after the application starts")
	}
	key := definition.method + " " + definition.pattern
	if _, exists := app.routes[key]; exists {
		return fmt.Errorf("route %s is already registered", key)
	}
	if err := app.registerHandler(key, app.handle(definition)); err != nil {
		return fmt.Errorf("register route %s: %w", key, err)
	}
	app.routes[key] = struct{}{}
	return nil
}

// Run serves the application until the context is cancelled or the server
// fails. The framework owns graceful shutdown.
func (app *App) Run(ctx context.Context) error {
	app.mu.Lock()
	if app.started {
		app.mu.Unlock()
		return errors.New("application has already started")
	}
	app.started = true
	app.mu.Unlock()

	server := &http.Server{
		Addr:              app.config.Server.Address,
		Handler:           http.HandlerFunc(app.serveHTTP),
		ReadHeaderTimeout: app.config.Server.readHeaderTimeout,
		ReadTimeout:       app.config.Server.readTimeout,
		WriteTimeout:      app.config.Server.writeTimeout,
		IdleTimeout:       app.config.Server.idleTimeout,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve application: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), app.config.Server.shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			shutdownErr := fmt.Errorf("shut down application: %w", err)
			closeErr := server.Close()
			serveErr := <-serverErrors
			if closeErr != nil {
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close application server: %w", closeErr))
			}
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve application: %w", serveErr))
			}
			return shutdownErr
		}
		err := <-serverErrors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve application: %w", err)
		}
		return nil
	}
}

func (app *App) registerHandler(pattern string, handler http.Handler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid or conflicting pattern: %v", recovered)
		}
	}()
	app.mux.Handle(pattern, handler)
	return nil
}

func (app *App) handle(definition routeDefinition) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		state, _ := request.Context().Value(requestStateKey{}).(requestState)
		requestContext := Context{request: request, principal: state.principal, pagination: app.config.Pagination}

		output, err := definition.invoke(requestContext)
		if err != nil {
			app.writeHandlerError(writer, request, err)
			return
		}
		if app.config.Pagination.Mode == policyRequired && definition.responseKind == responseSingular && isCollectionValue(output) {
			app.logger.Error("handler violated pagination policy", "method", request.Method, "path", request.URL.Path)
			writeError(writer, http.StatusInternalServerError, "internal_error", "The server could not process the request.")
			return
		}

		var payload []byte
		if definition.responseKind == responsePage {
			payload, err = json.Marshal(output)
		} else {
			payload, err = json.Marshal(struct {
				Data any `json:"data"`
			}{Data: output})
		}
		if err != nil {
			app.logger.Error("encode response", "method", request.Method, "path", request.URL.Path, "error", err)
			writeError(writer, http.StatusInternalServerError, "internal_error", "The server could not process the request.")
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(definition.status)
		payload = append(payload, '\n')
		if _, err := writer.Write(payload); err != nil {
			app.logger.Error("write response", "method", request.Method, "path", request.URL.Path, "error", err)
		}
	})
}

type requestStateKey struct{}

type requestState struct {
	principal Principal
}

func (app *App) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			app.logger.Error("request panicked", "method", request.Method, "path", request.URL.Path, "panic", recovered)
			writeError(writer, http.StatusInternalServerError, "internal_error", "The server could not process the request.")
		}
	}()

	writer.Header().Set("X-Content-Type-Options", "nosniff")
	request.Body = http.MaxBytesReader(writer, request.Body, app.config.Server.MaxBodyBytes)

	state := requestState{}
	if app.config.Authentication.Mode == policyRequired {
		principal, err := app.authenticator.Authenticate(request)
		if err != nil || principal.Subject == "" {
			writeError(writer, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		state.principal = principal
	}
	request = request.WithContext(context.WithValue(request.Context(), requestStateKey{}, state))
	_, pattern := app.mux.Handler(request)
	if pattern != "" {
		app.mux.ServeHTTP(writer, request)
		return
	}

	allowedMethods := app.allowedMethods(request)
	if len(allowedMethods) > 0 {
		writer.Header().Set("Allow", strings.Join(allowedMethods, ", "))
		writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "The request method is not allowed for this resource.")
		return
	}
	writeError(writer, http.StatusNotFound, "not_found", "The requested resource does not exist.")
}

func (app *App) allowedMethods(request *http.Request) []string {
	app.mu.Lock()
	registeredMethods := make(map[string]struct{}, len(app.routes))
	for key := range app.routes {
		method, _, _ := strings.Cut(key, " ")
		registeredMethods[method] = struct{}{}
	}
	app.mu.Unlock()

	allowed := make(map[string]struct{}, len(registeredMethods)+1)
	for method := range registeredMethods {
		candidate := request.Clone(request.Context())
		candidate.Method = method
		if _, pattern := app.mux.Handler(candidate); pattern == "" {
			continue
		}
		allowed[method] = struct{}{}
		if method == http.MethodGet {
			allowed[http.MethodHead] = struct{}{}
		}
	}

	methods := make([]string, 0, len(allowed))
	for method := range allowed {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func (app *App) writeHandlerError(writer http.ResponseWriter, request *http.Request, err error) {
	var httpError *HTTPError
	if errors.As(err, &httpError) && httpError.Status >= 400 && httpError.Status <= 599 && httpError.Code != "" && httpError.Message != "" {
		writeError(writer, httpError.Status, httpError.Code, httpError.Message)
		return
	}
	app.logger.Error("handler failed", "method", request.Method, "path", request.URL.Path, "error", err)
	writeError(writer, http.StatusInternalServerError, "internal_error", "The server could not process the request.")
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: message},
	})
}

func pageRequestFromContext(context Context) (PageRequest, error) {
	query := context.request.URL.Query()
	page, err := parsePositiveInteger(query.Get("page"), 1)
	if err != nil {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_page", "The page parameter must be a positive integer.")
	}
	size, err := parsePositiveInteger(query.Get("page_size"), contextPageDefault(context))
	if err != nil {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_page_size", "The page_size parameter must be a positive integer.")
	}
	maximum := contextPageMaximum(context)
	if size > maximum {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "page_size_too_large", fmt.Sprintf("The page_size parameter must not exceed %d.", maximum))
	}
	maximumInteger := int(^uint(0) >> 1)
	if page > 1 && page-1 > maximumInteger/size {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_page", "The requested page offset is too large.")
	}
	return PageRequest{Page: page, Size: size}, nil
}

func contextPageDefault(context Context) int {
	return context.pagination.DefaultSize
}

func contextPageMaximum(context Context) int {
	return context.pagination.MaximumSize
}

func parsePositiveInteger(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("value must be a positive integer")
	}
	return parsed, nil
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
