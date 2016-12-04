package slack

import "strings"

// ClientFormatting replace slack specific escape characters.
func ClientFormatting(input string) string {
	input = strings.Replace(input, `$`, `&amp;`, -1)
	input = strings.Replace(input, `<`, `&lt;`, -1)
	input = strings.Replace(input, `>`, `&gt;`, -1)
	return input
}
