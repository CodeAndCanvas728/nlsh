package validate

import (
	"regexp"
	"strconv"
)

// itoa is strconv.Itoa, kept short because it is used a lot below.
func itoa(n int) string { return strconv.Itoa(n) }

// urlPattern matches a URL so that its path is not mistaken for a file the
// plan has to mention.
var urlPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S+`)

// pathPattern matches a path in a request.
//
// The leading boundary matters: without it "and/or" would yield the path
// "/or" and every request containing the word "and" would fail validation.
// A path has to start after a space, a quote or the start of the request.
var pathPattern = regexp.MustCompile(`(?:^|[\s"'(\[{,;])((?:~|\.{1,2})?/[A-Za-z0-9._~+@%=-]+(?:/[A-Za-z0-9._~+@%=-]*)*)`)

// PathsMentioned returns the paths a request refers to.
//
// Paths inside URLs are ignored: "fetch https://example.com/data" is a
// request about a URL, not about the local file /data.
func PathsMentioned(request string) []string {
	cleaned := urlPattern.ReplaceAllString(request, " ")

	var paths []string
	seen := map[string]bool{}
	for _, m := range pathPattern.FindAllStringSubmatch(cleaned, -1) {
		path := m[1]
		if path == "/" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}
