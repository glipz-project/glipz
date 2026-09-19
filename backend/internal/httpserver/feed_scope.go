package httpserver

import "strings"

func normalizeFeedScope(raw string) (string, bool) {
	switch scope := strings.ToLower(strings.TrimSpace(raw)); scope {
	case "", "all":
		return "all", true
	case "following", "recommended":
		return scope, true
	default:
		return "", false
	}
}
