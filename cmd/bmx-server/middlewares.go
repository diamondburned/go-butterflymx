//go:build goexperiment.jsonv2

package main

import (
	"context"
	"fmt"
	"net/http"
)

// ContextMiddleware injects the default or request-header ButterflyMX API token into the request context.
func ContextMiddleware(defaultToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-ButterflyMX-API-Token")
			if token == "" {
				token = defaultToken
			}
			ctx := context.WithValue(r.Context(), tokenContextKey, token)
			w.Header().Set("Server", "bmx-server")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthMiddleware authenticates requests against a configured API secret.
func AuthMiddleware(apiSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiSecret == "" {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			key := r.Header.Get("X-API-Key")
			if auth != "Bearer "+apiSecret && key != apiSecret {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"Unauthorized: invalid or missing API secret"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
