package arr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// itoa formats an int for use in a URL path.
func itoa(i int) string { return strconv.Itoa(i) }

// btoa formats a bool for use in a query parameter.
func btoa(b bool) string { return strconv.FormatBool(b) }

// joinInts renders ints as a comma-separated query value.
func joinInts(xs []int) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, strconv.Itoa(x))
	}
	return strings.Join(parts, ",")
}

// unmarshal decodes a JSON payload, wrapping failures with context.
func unmarshal(body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}
