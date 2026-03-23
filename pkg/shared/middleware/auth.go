// Package middleware provides shared HTTP middleware for OpsNexus services.
package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	UserRolesKey contextKey = "user_roles"
	OrgIDKey     contextKey = "org_id"

	headerUserID    = "X-User-Id"
	headerUserRoles = "X-User-Roles"
	headerOrgID     = "X-Org-Id"
	headerAuth      = "Authorization"
)

// UserContext holds the authenticated user's context.
type UserContext struct {
	UserID string
	OrgID  string
	Roles  []string
}

// Auth middleware extracts user context from gateway-injected headers.
// In production, the API gateway validates the JWT and sets these headers.
// Services behind the gateway trust these headers.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(headerUserID)
		if userID == "" {
			http.Error(w, `{"code":"UNAUTHORIZED","message":"missing user context"}`, http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, OrgIDKey, r.Header.Get(headerOrgID))

		rolesHeader := r.Header.Get(headerUserRoles)
		if rolesHeader != "" {
			roles := strings.Split(rolesHeader, ",")
			ctx = context.WithValue(ctx, UserRolesKey, roles)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserContext extracts the UserContext from a request context.
func GetUserContext(ctx context.Context) UserContext {
	uc := UserContext{}
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		uc.UserID = v
	}
	if v, ok := ctx.Value(OrgIDKey).(string); ok {
		uc.OrgID = v
	}
	if v, ok := ctx.Value(UserRolesKey).([]string); ok {
		uc.Roles = v
	}
	return uc
}

// RequireRole returns middleware that checks if the user has at least one of the required roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoles, _ := r.Context().Value(UserRolesKey).([]string)

			for _, ur := range userRoles {
				if _, ok := roleSet[ur]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"code":"FORBIDDEN","message":"insufficient permissions"}`, http.StatusForbidden)
		})
	}
}
