package controller

import "fmt"

// importSummary — the message of an import answer: "3 partners created, 2 updated".
func importSummary(created, updated int, noun string) string {
	plural := func(n int) string {
		if n == 1 {
			return noun
		}
		return noun + "s"
	}
	switch {
	case updated == 0:
		return fmt.Sprintf("%d %s created", created, plural(created))
	case created == 0:
		return fmt.Sprintf("%d %s updated", updated, plural(updated))
	default:
		return fmt.Sprintf("%d %s created, %d updated", created, plural(created), updated)
	}
}
