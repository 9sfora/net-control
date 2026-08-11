package httpserver

import (
	"net"
	"net/http"
	"strings"
)

func routeByHost(hosts VirtualHosts, application, acs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		}
		switch host {
		case hosts.Application:
			application.ServeHTTP(w, r)
		case hosts.ACS:
			acs.ServeHTTP(w, r)
		default:
			WriteError(w, r, http.StatusMisdirectedRequest, "MISDIRECTED_REQUEST", "Unknown host")
		}
	})
}
