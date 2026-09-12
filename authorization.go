package gopinion

import (
	"context"
	"errors"
	"fmt"
)

// ErrAuthorizationRequired indicates that a route has no authorization phase
// while authorization is globally required.
var ErrAuthorizationRequired = errors.New("authorization is required")

// ErrForbidden indicates that an authorization decision denied the request.
var ErrForbidden = errors.New("request is forbidden")

// AuthorizationDecision is the result returned by an Authorizer.
type AuthorizationDecision uint8

const (
	// Deny rejects an authorization request.
	Deny AuthorizationDecision = iota
	// Allow permits an authorization request.
	Allow
)

// AuthorizationResource identifies a domain resource and the trusted
// attributes needed to authorize access to it.
type AuthorizationResource struct {
	Type       string
	ID         string
	Attributes map[string]any
}

// AuthorizationRequest describes the action and resource to authorize.
// Context contains trusted request-scoped facts, not context.Context values.
type AuthorizationRequest struct {
	Action   string
	Resource AuthorizationResource
	Context  map[string]any
}

// AuthorizationPlan carries an authorization request and the prepared domain
// value made available to the handler only after an Allow decision. Preparation
// runs before authorization and must not mutate durable or external state.
type AuthorizationPlan[T any] struct {
	Request AuthorizationRequest
	Value   T
}

// Authorizer decides whether a principal may perform an action on a resource.
type Authorizer interface {
	Authorize(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error)
}

// AuthorizerFunc adapts a function to Authorizer.
type AuthorizerFunc func(context.Context, Principal, AuthorizationRequest) (AuthorizationDecision, error)

// Authorize calls f with the authorization request.
func (f AuthorizerFunc) Authorize(ctx context.Context, principal Principal, request AuthorizationRequest) (AuthorizationDecision, error) {
	return f(ctx, principal, request)
}

func authorizePlan[T any](ctx Context, authorizer Authorizer, plan AuthorizationPlan[T]) (T, error) {
	var zero T
	if plan.Request.Action == "" {
		return zero, errors.New("authorization action must not be empty")
	}
	if plan.Request.Resource.Type == "" {
		return zero, errors.New("authorization resource type must not be empty")
	}
	if plan.Request.Resource.ID == "" {
		return zero, errors.New("authorization resource ID must not be empty")
	}

	decision, err := authorizer.Authorize(ctx.request.Context(), ctx.principal, plan.Request)
	if err != nil {
		// Do not unwrap evaluator errors: an evaluator returning ErrForbidden is
		// still an infrastructure failure, not a Deny decision.
		return zero, authorizationEvaluationError{
			action:       plan.Request.Action,
			resourceType: plan.Request.Resource.Type,
			resourceID:   plan.Request.Resource.ID,
			err:          err,
		}
	}
	switch decision {
	case Allow:
		return plan.Value, nil
	case Deny:
		return zero, ErrForbidden
	default:
		return zero, fmt.Errorf("authorizer returned invalid decision %d", decision)
	}
}

type authorizationEvaluationError struct {
	action       string
	resourceType string
	resourceID   string
	err          error
}

func (err authorizationEvaluationError) Error() string {
	return fmt.Sprintf("authorize %s on %s %q: %v", err.action, err.resourceType, err.resourceID, err.err)
}
