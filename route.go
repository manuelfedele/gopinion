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
)

type routeDefinition struct {
	method             string
	pattern            string
	status             int
	responseKind       responseKind
	responseCollection bool
	invoke             func(Context) (any, error)
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

// Get creates a singular GET route.
func Get[O any](pattern string, handler func(Context) (O, error)) Route {
	var invoke func(Context) (any, error)
	if handler != nil {
		invoke = func(context Context) (any, error) {
			return handler(context)
		}
	}
	return route{definition: routeDefinition{
		method:             http.MethodGet,
		pattern:            pattern,
		status:             http.StatusOK,
		responseKind:       responseSingular,
		responseCollection: isCollectionType(reflect.TypeFor[O]()),
		invoke:             invoke,
	}}
}

// Post creates a singular POST route with a strictly decoded JSON body.
func Post[I, O any](pattern string, handler func(Context, I) (O, error)) Route {
	var invoke func(Context) (any, error)
	if handler != nil {
		invoke = func(context Context) (any, error) {
			var input I
			decoder := json.NewDecoder(context.request.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&input); err != nil {
				var maximumBytesError *http.MaxBytesError
				if errors.As(err, &maximumBytesError) {
					return nil, NewHTTPError(http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.")
				}
				return nil, NewHTTPError(http.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
			}

			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				return nil, NewHTTPError(http.StatusBadRequest, "invalid_json", "The request body must contain exactly one JSON value.")
			}
			return handler(context, input)
		}
	}
	return route{definition: routeDefinition{
		method:             http.MethodPost,
		pattern:            pattern,
		status:             http.StatusCreated,
		responseKind:       responseSingular,
		responseCollection: isCollectionType(reflect.TypeFor[O]()),
		invoke:             invoke,
	}}
}

// List creates a paginated GET route. Its handler cannot return an unpaginated
// collection.
func List[O any](pattern string, handler func(Context, PageRequest) (Page[O], error)) Route {
	var invoke func(Context) (any, error)
	if handler != nil {
		invoke = func(context Context) (any, error) {
			request, err := pageRequestFromContext(context)
			if err != nil {
				return nil, err
			}
			page, err := handler(context, request)
			if err != nil {
				return nil, err
			}
			if err := page.validate(); err != nil {
				return nil, fmt.Errorf("invalid page returned by handler: %w", err)
			}
			return page, nil
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
