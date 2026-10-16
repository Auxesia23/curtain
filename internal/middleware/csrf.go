package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// CSRFProtect guards state-changing endpoints against CSRF attacks using
// HttpOnly session cookies.
//
// Cookie-authenticated requests are allowed only when the browser marks them
// as same-origin (`Sec-Fetch-Site`) or the `Origin` host matches the request
// host. Bearer-token CLI requests are unaffected because browsers do not send
// those tokens automatically.
func CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if _, err := r.Cookie(SessionCookieName); err != nil {
			// No session cookie: this is a CLI bearer-token or public request.
			next.ServeHTTP(w, r)
			return
		}

		if r.Header.Get("Sec-Fetch-Site") == "same-origin" {
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			if parsed, err := url.Parse(origin); err == nil && strings.EqualFold(parsed.Host, r.Host) {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeJSONError(w, http.StatusForbidden, "csrf check failed: origin does not match")
	})
}
