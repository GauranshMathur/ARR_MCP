package arr

import (
	"context"
	"encoding/json"
	"fmt"
)

// Rules cross the MCP boundary only as Maintainerr's YAML. Its encode and
// decode endpoints are pure transforms, so the numeric rule encoding
// (firstVal: [app, property], action indexes) never reaches the model, and
// Maintainerr's own parser is what validates a condition.

// maintainerrEncodeRules renders rule objects as Maintainerr's YAML. The
// endpoint takes the rules as a JSON string, not an array.
func maintainerrEncodeRules(ctx context.Context, c *Client, rules []json.RawMessage, mediaType string) (string, error) {
	encoded, err := json.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("encoding rules: %w", err)
	}
	body, err := c.Post(ctx, "/rules/yaml/encode", struct {
		Rules     string `json:"rules"`
		MediaType string `json:"mediaType"`
	}{string(encoded), mediaType})
	if err != nil {
		return "", err
	}
	st, err := maintainerrDecodeStatus(body, "render the rules as YAML")
	if err != nil {
		return "", err
	}
	return st.Result, nil
}

// maintainerrDecodeRules parses Maintainerr YAML into rule objects. A rule it
// could not resolve is dropped by Maintainerr, which would widen what the
// group matches, so any skipped rule fails the whole decode.
func maintainerrDecodeRules(ctx context.Context, c *Client, yaml, mediaType string) ([]json.RawMessage, error) {
	body, err := c.Post(ctx, "/rules/yaml/decode", struct {
		YAML      string `json:"yaml"`
		MediaType string `json:"mediaType"`
	}{yaml, mediaType})
	if err != nil {
		return nil, err
	}
	st, err := maintainerrDecodeStatus(body, "read rulesYaml")
	if err != nil {
		return nil, err
	}
	if st.Skipped > 0 {
		return nil, fmt.Errorf("maintainerr could not resolve %d rule(s) in rulesYaml; nothing was saved, "+
			"because dropping a condition widens what the rule group matches", st.Skipped)
	}
	var decoded struct {
		Rules []json.RawMessage `json:"rules"`
	}
	if err := unmarshal([]byte(st.Result), &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Rules) == 0 {
		return nil, fmt.Errorf("rulesYaml contains no rules")
	}
	return decoded.Rules, nil
}
