package main

import "net/http"

var cspHeader = ""

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strict-Transport-Security
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

		// Content-Security-Policy
		w.Header().Set("Content-Security-Policy", cspHeader)

		// X-Frame-Options
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")

		// X-Content-Type-Options
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Cross-Origin-Embedder-Policy
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")

		// Cross-Origin-Opener-Policy
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		// Cross-Origin-Resource-Policy
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

		// Referrer-Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()")

		next.ServeHTTP(w, r)
	})
}
