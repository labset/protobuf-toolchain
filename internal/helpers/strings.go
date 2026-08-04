// Package helpers holds small, dependency-free utilities shared across the
// toolchain, such as identifier case conversions.
package helpers

import "strings"

// ToSnake converts a CamelCase identifier to snake_case (User -> user,
// OrderItem -> order_item).
func ToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
