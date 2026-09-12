package gopinion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ErrPaginationRequired indicates that a singular route attempted to return a
// collection while pagination is globally required.
var ErrPaginationRequired = errors.New("pagination is required for collection responses")

const configFileName = "gopinion.yaml"

type options struct {
	authenticator Authenticator
	authorizer    Authorizer
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

// WithAuthorizer supplies the implementation used by authorized routes.
func WithAuthorizer(authorizer Authorizer) Option {
	return optionFunc(func(options *options) error {
		if isNil(authorizer) {
			return errors.New("authorizer must not be nil")
		}
		options.authorizer = authorizer
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
	authorizer    Authorizer
	logger        *slog.Logger
	mux           *http.ServeMux

	mu      sync.Mutex
	started bool
	routes  map[string]struct{}
}

// New loads gopinion.yaml from the current working directory and constructs the framework.
func New(suppliedOptions ...Option) (*App, error) {
	return newApp(configFileName, suppliedOptions...)
}

func newApp(configPath string, suppliedOptions ...Option) (*App, error) {
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
	if cfg.Authorization.Mode == policyRequired && isNil(configuredOptions.authorizer) {
		return nil, errors.New("authorization is required but no authorizer was supplied")
	}

	return &App{
		config:        cfg,
		authenticator: configuredOptions.authenticator,
		authorizer:    configuredOptions.authorizer,
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
	if definition.authorizationDeclared && !definition.authorizationValid {
		return errors.New("route authorization phase must not be nil")
	}
	if app.config.Authorization.Mode == policyRequired && !definition.authorizationDeclared {
		return fmt.Errorf("%s %s: %w", definition.method, definition.pattern, ErrAuthorizationRequired)
	}
	if definition.authorizationDeclared && isNil(app.authorizer) {
		return errors.New("authorized route requires an authorizer")
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
	if ctx == nil {
		return errors.New("run context must not be nil")
	}

	app.mu.Lock()
	if app.started {
		app.mu.Unlock()
		return errors.New("application has already started")
	}
	app.started = true
	app.mu.Unlock()

	server := app.newServer()
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

func (app *App) newServer() *http.Server {
	return &http.Server{
		Addr:              app.config.Server.Address,
		Handler:           http.HandlerFunc(app.serveHTTP),
		ReadHeaderTimeout: app.config.Server.readHeaderTimeout,
		ReadTimeout:       app.config.Server.readTimeout,
		WriteTimeout:      app.config.Server.writeTimeout,
		IdleTimeout:       app.config.Server.idleTimeout,
		// Keep OPTIONS * inside the authenticated framework-owned HTTP surface.
		DisableGeneralOptionsHandler: true,
	}
}

func (app *App) registerHandler(pattern string, handler http.Handler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid or conflicting pattern: %v", recovered)
		}
	}()
	app.mux.Handle(pattern, registeredRouteHandler{Handler: handler})
	return nil
}

// registeredRouteHandler distinguishes application routes from redirect
// handlers synthesized internally by http.ServeMux.
type registeredRouteHandler struct {
	http.Handler
}

func (app *App) handle(definition routeDefinition) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		state, _ := request.Context().Value(requestStateKey{}).(requestState)
		requestContext := Context{request: request, principal: state.principal, pagination: app.config.Pagination}

		output, err := definition.invoke(requestContext, app.authorizer)
		if err != nil {
			app.writeHandlerError(writer, request, err)
			return
		}
		if app.config.Pagination.Mode == policyRequired && definition.responseKind == responseSingular && isCollectionValue(output) {
			app.logger.Error("handler violated pagination policy", "method", request.Method, "path", request.URL.Path)
			writeError(writer, http.StatusInternalServerError, "internal_error", "The server could not process the request.")
			return
		}

		if definition.responseKind == responseEmpty {
			writer.WriteHeader(definition.status)
			return
		}

		var payload []byte
		var linkHeader string
		if definition.responseKind == responsePage {
			payload, err = json.Marshal(output)
			if err == nil {
				linkHeader = paginationLinkHeader(request, output.(pageValue), app.config.Pagination.MaximumOffset)
			}
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
		if linkHeader != "" {
			writer.Header().Set("Link", linkHeader)
		}
		writer.WriteHeader(definition.status)
		if request.Method == http.MethodHead {
			return
		}
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
		if errors.Is(err, ErrUnauthenticated) || err == nil && principal.Subject == "" {
			writer.Header().Set("WWW-Authenticate", app.authenticationChallenge())
			writeError(writer, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		if err != nil {
			app.logger.Error("authenticate request", "method", request.Method, "path", request.URL.Path, "error", err)
			writeError(writer, http.StatusInternalServerError, "internal_error", "The server could not process the request.")
			return
		}
		state.principal = principal
	}
	request = request.WithContext(context.WithValue(request.Context(), requestStateKey{}, state))
	_, pattern := app.mux.Handler(request)
	if pattern != "" {
		app.mux.ServeHTTP(&redirectRejectingResponseWriter{ResponseWriter: writer}, request)
		return
	}

	allowedMethods := app.allowedMethods(request)
	if request.Method == http.MethodOptions && len(allowedMethods) > 0 {
		writer.Header().Set("Allow", strings.Join(allowedMethods, ", "))
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if len(allowedMethods) > 0 {
		writer.Header().Set("Allow", strings.Join(allowedMethods, ", "))
		writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "The request method is not allowed for this resource.")
		return
	}
	writeError(writer, http.StatusNotFound, "not_found", "The requested resource does not exist.")
}

func (app *App) authenticationChallenge() string {
	if challenger, ok := app.authenticator.(AuthenticationChallenger); ok {
		if challenge := challenger.AuthenticationChallenge(); challenge != "" {
			return challenge
		}
	}
	return "Bearer"
}

type redirectRejectingResponseWriter struct {
	http.ResponseWriter
	rejected bool
}

func (writer *redirectRejectingResponseWriter) WriteHeader(status int) {
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest {
		writer.Header().Del("Location")
		writer.rejected = true
		writeError(writer.ResponseWriter, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		return
	}
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *redirectRejectingResponseWriter) Write(payload []byte) (int, error) {
	if writer.rejected {
		return len(payload), nil
	}
	return writer.ResponseWriter.Write(payload)
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
		handler, pattern := app.mux.Handler(candidate)
		if _, registered := handler.(registeredRouteHandler); pattern == "" || !registered {
			continue
		}
		allowed[method] = struct{}{}
		if method == http.MethodGet {
			allowed[http.MethodHead] = struct{}{}
		}
	}

	methods := make([]string, 0, len(allowed))
	if len(allowed) > 0 {
		allowed[http.MethodOptions] = struct{}{}
	}
	for method := range allowed {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func (app *App) writeHandlerError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, ErrForbidden) {
		writeError(writer, http.StatusForbidden, "forbidden", "The request is not authorized.")
		return
	}
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
	query, err := url.ParseQuery(context.request.URL.RawQuery)
	if err != nil {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_query", "The query string is not valid.")
	}
	if _, exists := query["page"]; exists {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_pagination", "Use the limit and offset parameters for pagination.")
	}
	if _, exists := query["page_size"]; exists {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_pagination", "Use the limit and offset parameters for pagination.")
	}
	if len(query["limit"]) > 1 {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_limit", "The limit parameter must be specified at most once.")
	}
	if len(query["offset"]) > 1 {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_offset", "The offset parameter must be specified at most once.")
	}
	if values, exists := query["limit"]; exists && values[0] == "" {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_limit", "The limit parameter must be a positive integer.")
	}
	if values, exists := query["offset"]; exists && values[0] == "" {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_offset", "The offset parameter must be a non-negative integer.")
	}
	limit, err := parsePositiveInteger(query.Get("limit"), contextDefaultLimit(context))
	if err != nil {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_limit", "The limit parameter must be a positive integer.")
	}
	offset, err := parseNonNegativeInteger(query.Get("offset"), 0)
	if err != nil {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "invalid_offset", "The offset parameter must be a non-negative integer.")
	}
	maximum := contextMaximumLimit(context)
	if limit > maximum {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "limit_too_large", fmt.Sprintf("The limit parameter must not exceed %d.", maximum))
	}
	maximumOffset := contextMaximumOffset(context)
	if offset > maximumOffset {
		return PageRequest{}, NewHTTPError(http.StatusBadRequest, "offset_too_large", fmt.Sprintf("The offset parameter must not exceed %d.", maximumOffset))
	}
	return PageRequest{Limit: limit, Offset: offset}, nil
}

func contextDefaultLimit(context Context) int {
	return context.pagination.DefaultLimit
}

func contextMaximumLimit(context Context) int {
	return context.pagination.MaximumLimit
}

func contextMaximumOffset(context Context) int {
	return context.pagination.MaximumOffset
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

func parseNonNegativeInteger(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, errors.New("value must be a non-negative integer")
	}
	return parsed, nil
}

func paginationLinkHeader(request *http.Request, page pageValue, maximumOffset int) string {
	pagination, total := page.pagination()
	if total == 0 {
		return ""
	}

	limit := int64(pagination.Limit)
	offset := int64(pagination.Offset)
	lastOffset := (total - 1) / limit * limit
	links := []string{paginationLink(request.URL, pagination.Limit, 0, "first")}
	if offset > 0 {
		previousOffset := offset - limit
		if previousOffset < 0 {
			previousOffset = 0
		}
		links = append(links, paginationLink(request.URL, pagination.Limit, previousOffset, "prev"))
	}
	if nextOffset := offset + limit; offset < total && limit < total-offset && nextOffset <= int64(maximumOffset) {
		links = append(links, paginationLink(request.URL, pagination.Limit, nextOffset, "next"))
	}
	if lastOffset <= int64(maximumOffset) {
		links = append(links, paginationLink(request.URL, pagination.Limit, lastOffset, "last"))
	}
	return strings.Join(links, ", ")
}

func paginationLink(requestURL *url.URL, limit int, offset int64, relation string) string {
	target := *requestURL
	target.Scheme = ""
	target.Host = ""
	target.User = nil
	target.Fragment = ""
	target.RawFragment = ""
	query := target.Query()
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.FormatInt(offset, 10))
	target.RawQuery = query.Encode()
	return fmt.Sprintf("<%s>; rel=\"%s\"", target.String(), relation)
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
