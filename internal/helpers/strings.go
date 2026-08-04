package helpers

import "strings"

// ToSnake converts a CamelCase identifier to snake_case. It is acronym-aware:
// a run of capitals is treated as a single word, with a boundary inserted only
// where a new word begins.
//
//	User       -> user
//	OrderItem  -> order_item
//	APIKey     -> api_key
//	HTTPServer -> http_server
//	S3Bucket   -> s3_bucket
func ToSnake(s string) string {
	runes := []rune(s)

	var b strings.Builder
	b.Grow(len(runes) + 4)

	for i, r := range runes {
		if !isUpper(r) {
			b.WriteRune(r)
			continue
		}
		if i > 0 && boundaryBefore(runes, i) {
			b.WriteByte('_')
		}
		b.WriteRune(r - 'A' + 'a')
	}

	return b.String()
}

// boundaryBefore reports whether an underscore should precede the uppercase rune
// at index i: after a lowercase letter or digit (fooBar, s3Bucket), or at the
// tail of an acronym that starts a new word (APIKey -> api_key).
func boundaryBefore(runes []rune, i int) bool {
	prev := runes[i-1]
	if isLower(prev) || isDigit(prev) {
		return true
	}

	return isUpper(prev) && i+1 < len(runes) && isLower(runes[i+1])
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
func isDigit(r rune) bool { return r >= '0' && r <= '9' }
