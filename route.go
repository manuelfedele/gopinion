package gopinion

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
)

type responseKind uint8

const (
	responseSingular responseKind = iota
	responsePage
	responseEmpty
)

type routeDefinition struct {
	method                string
	pattern               string
	status                int
	responseKind          responseKind
	responseCollection    bool
	authorizationDeclared bool
	authorizationValid    bool
	invoke                func(Context, Authorizer) (any, error)
}

// Route is a framework-owned endpoint definition. Only GOpinion route
// constructors can implement it.
type Route interface {
	gopinionRoute() routeDefinition
}

type route struct {
	definition routeDefinition
}

func (route route) gopinionRoute() routeDefinition {
	return route.definition
}

// Get creates a singular GET route without an authorization phase. It can only
// be registered when global authorization is disabled.
func Get[O any](pattern string, handler func(Context) (O, error)) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, _ Authorizer) (any, error) {
			return handler(ctx)
		}
	}
	return singularRoute(http.MethodGet, pattern, http.StatusOK, reflect.TypeFor[O](), invoke)
}

// AuthorizedGet creates a singular GET route with a typed authorization phase.
func AuthorizedGet[A, O any](pattern string, prepare func(Context) (AuthorizationPlan[A], error), handler func(Context, A) (O, error)) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, authorizer Authorizer) (any, error) {
			plan, err := prepare(ctx)
			if err != nil {
				return nil, err
			}
			value, err := authorizePlan(ctx, authorizer, plan)
			if err != nil {
				return nil, err
			}
			return handler(ctx, value)
		}
	}
	return authorizedSingularRoute(http.MethodGet, pattern, http.StatusOK, reflect.TypeFor[O](), prepare != nil, invoke)
}

// Post creates a singular POST route with a strictly decoded JSON body and no
// authorization phase. It can only be registered when global authorization is disabled.
func Post[I, O any](pattern string, handler func(Context, I) (O, error)) Route {
	return bodyRoute(http.MethodPost, pattern, http.StatusCreated, handler)
}

// AuthorizedPost creates a singular POST route with strict JSON decoding and a
// typed authorization phase.
func AuthorizedPost[I, A, O any](pattern string, prepare func(Context, I) (AuthorizationPlan[A], error), handler func(Context, I, A) (O, error)) Route {
	return authorizedBodyRoute(http.MethodPost, pattern, http.StatusCreated, prepare, handler)
}

// Put creates a singular PUT route with a strictly decoded JSON body and no
// authorization phase. It can only be registered when global authorization is disabled.
func Put[I, O any](pattern string, handler func(Context, I) (O, error)) Route {
	return bodyRoute(http.MethodPut, pattern, http.StatusOK, handler)
}

// AuthorizedPut creates a singular PUT route with strict JSON decoding and a
// typed authorization phase.
func AuthorizedPut[I, A, O any](pattern string, prepare func(Context, I) (AuthorizationPlan[A], error), handler func(Context, I, A) (O, error)) Route {
	return authorizedBodyRoute(http.MethodPut, pattern, http.StatusOK, prepare, handler)
}

// Patch creates a singular PATCH route with a strictly decoded JSON body and
// no authorization phase. It can only be registered when global authorization is disabled.
func Patch[I, O any](pattern string, handler func(Context, I) (O, error)) Route {
	return bodyRoute(http.MethodPatch, pattern, http.StatusOK, handler)
}

// AuthorizedPatch creates a singular PATCH route with strict JSON decoding and
// a typed authorization phase.
func AuthorizedPatch[I, A, O any](pattern string, prepare func(Context, I) (AuthorizationPlan[A], error), handler func(Context, I, A) (O, error)) Route {
	return authorizedBodyRoute(http.MethodPatch, pattern, http.StatusOK, prepare, handler)
}

// Delete creates a DELETE route returning 204 with no response body and no
// authorization phase. It can only be registered when global authorization is disabled.
func Delete(pattern string, handler func(Context) error) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, _ Authorizer) (any, error) {
			return nil, handler(ctx)
		}
	}
	return route{definition: routeDefinition{
		method:       http.MethodDelete,
		pattern:      pattern,
		status:       http.StatusNoContent,
		responseKind: responseEmpty,
		invoke:       invoke,
	}}
}

// AuthorizedDelete creates a DELETE route with a typed authorization phase. A
// successful request returns 204 with no response body.
func AuthorizedDelete[A any](pattern string, prepare func(Context) (AuthorizationPlan[A], error), handler func(Context, A) error) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, authorizer Authorizer) (any, error) {
			plan, err := prepare(ctx)
			if err != nil {
				return nil, err
			}
			value, err := authorizePlan(ctx, authorizer, plan)
			if err != nil {
				return nil, err
			}
			return nil, handler(ctx, value)
		}
	}
	return route{definition: routeDefinition{
		method:                http.MethodDelete,
		pattern:               pattern,
		status:                http.StatusNoContent,
		responseKind:          responseEmpty,
		authorizationDeclared: true,
		authorizationValid:    prepare != nil,
		invoke:                invoke,
	}}
}

// List creates a paginated GET route without an authorization phase. Its
// handler cannot return an unpaginated collection, and it can only be
// registered when global authorization is disabled.
func List[O any](pattern string, handler func(Context, PageRequest) (Page[O], error)) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, _ Authorizer) (any, error) {
			request, err := pageRequestFromContext(ctx)
			if err != nil {
				return nil, err
			}
			return invokeListHandler(ctx, request, handler)
		}
	}
	return route{definition: routeDefinition{
		method:       http.MethodGet,
		pattern:      pattern,
		status:       http.StatusOK,
		responseKind: responsePage,
		invoke:       invoke,
	}}
}

// AuthorizedList creates a paginated GET route with a typed authorization
// phase. The prepared value can carry an authorization scope used by the query.
func AuthorizedList[A, O any](pattern string, prepare func(Context, PageRequest) (AuthorizationPlan[A], error), handler func(Context, PageRequest, A) (Page[O], error)) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, authorizer Authorizer) (any, error) {
			request, err := pageRequestFromContext(ctx)
			if err != nil {
				return nil, err
			}
			plan, err := prepare(ctx, request)
			if err != nil {
				return nil, err
			}
			value, err := authorizePlan(ctx, authorizer, plan)
			if err != nil {
				return nil, err
			}
			return invokeListHandlerWithAuthorization(ctx, request, value, handler)
		}
	}
	return route{definition: routeDefinition{
		method:                http.MethodGet,
		pattern:               pattern,
		status:                http.StatusOK,
		responseKind:          responsePage,
		authorizationDeclared: true,
		authorizationValid:    prepare != nil,
		invoke:                invoke,
	}}
}

func singularRoute(method, pattern string, status int, outputType reflect.Type, invoke func(Context, Authorizer) (any, error)) Route {
	return route{definition: routeDefinition{
		method:             method,
		pattern:            pattern,
		status:             status,
		responseKind:       responseSingular,
		responseCollection: isCollectionType(outputType),
		invoke:             invoke,
	}}
}

func authorizedSingularRoute(method, pattern string, status int, outputType reflect.Type, valid bool, invoke func(Context, Authorizer) (any, error)) Route {
	result := singularRoute(method, pattern, status, outputType, invoke).gopinionRoute()
	result.authorizationDeclared = true
	result.authorizationValid = valid
	return route{definition: result}
}

func bodyRoute[I, O any](method, pattern string, status int, handler func(Context, I) (O, error)) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, _ Authorizer) (any, error) {
			input, err := decodeJSONBody[I](ctx)
			if err != nil {
				return nil, err
			}
			return handler(ctx, input)
		}
	}
	return singularRoute(method, pattern, status, reflect.TypeFor[O](), invoke)
}

func authorizedBodyRoute[I, A, O any](method, pattern string, status int, prepare func(Context, I) (AuthorizationPlan[A], error), handler func(Context, I, A) (O, error)) Route {
	var invoke func(Context, Authorizer) (any, error)
	if handler != nil {
		invoke = func(ctx Context, authorizer Authorizer) (any, error) {
			input, err := decodeJSONBody[I](ctx)
			if err != nil {
				return nil, err
			}
			plan, err := prepare(ctx, input)
			if err != nil {
				return nil, err
			}
			value, err := authorizePlan(ctx, authorizer, plan)
			if err != nil {
				return nil, err
			}
			return handler(ctx, input, value)
		}
	}
	return authorizedSingularRoute(method, pattern, status, reflect.TypeFor[O](), prepare != nil, invoke)
}

func decodeJSONBody[I any](ctx Context) (I, error) {
	var input I
	decoder := json.NewDecoder(ctx.request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		var maximumBytesError *http.MaxBytesError
		if errors.As(err, &maximumBytesError) {
			return input, NewHTTPError(http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.")
		}
		return input, NewHTTPError(http.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		var maximumBytesError *http.MaxBytesError
		if errors.As(err, &maximumBytesError) {
			return input, NewHTTPError(http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.")
		}
		return input, NewHTTPError(http.StatusBadRequest, "invalid_json", "The request body must contain exactly one JSON value.")
	}
	return input, nil
}

func invokeListHandler[O any](ctx Context, request PageRequest, handler func(Context, PageRequest) (Page[O], error)) (any, error) {
	page, err := handler(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := page.validate(); err != nil {
		return nil, fmt.Errorf("invalid page returned by handler: %w", err)
	}
	return page, nil
}

func invokeListHandlerWithAuthorization[A, O any](ctx Context, request PageRequest, value A, handler func(Context, PageRequest, A) (Page[O], error)) (any, error) {
	page, err := handler(ctx, request, value)
	if err != nil {
		return nil, err
	}
	if err := page.validate(); err != nil {
		return nil, fmt.Errorf("invalid page returned by handler: %w", err)
	}
	return page, nil
}

type pageValue interface {
	pageMarker()
}

func isCollectionType(valueType reflect.Type) bool {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType.Implements(reflect.TypeFor[pageValue]()) {
		return true
	}
	switch valueType.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return true
	default:
		return false
	}
}

func isCollectionValue(value any) bool {
	if value == nil {
		return false
	}
	if _, ok := value.(pageValue); ok {
		return true
	}
	return isCollectionType(reflect.TypeOf(value))
}
