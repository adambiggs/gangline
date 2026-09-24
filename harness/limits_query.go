package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

// QueryProviderLimits asks the native account service without creating a thread
// or sending a prompt. No recorded session reading can satisfy this query.
func QueryProviderLimits(ctx context.Context, collar Collar) ([]LimitWindow, error) {
	inv := collar.Primitives.LimitsQuery
	if inv == nil {
		return nil, fmt.Errorf("collar %q does not declare an agent-free limits query; use gang limits NAME for an observed agent reading", collar.Name)
	}
	if inv.Name != "codex-app-server-limits" {
		return nil, fmt.Errorf("unknown limits-query primitive %q", inv.Name)
	}
	child := exec.CommandContext(ctx, collar.Launch.Command, "app-server", "--listen", "stdio://")
	child.Env = os.Environ()
	for key, value := range collar.Launch.Env {
		child.Env = append(child.Env, key+"="+value)
	}
	stdin, err := child.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := child.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start %s limits query: %w", collar.Name, err)
	}
	limits, queryErr := queryCodexLimits(stdout, stdin)
	// EOF shuts down this private server. The caller's context also bounds a
	// server that does not close after EOF; Wait reaps it on every path.
	_ = stdin.Close()
	err = child.Wait()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s limits query: %w", collar.Name, ctx.Err())
	}
	if queryErr != nil {
		return nil, queryErr
	}
	if err != nil {
		return nil, fmt.Errorf("%s limits query exited: %w", collar.Name, err)
	}
	return limits, nil
}

func queryCodexLimits(input io.Reader, output io.Writer) ([]LimitWindow, error) {
	decoder := json.NewDecoder(io.LimitReader(input, 1024*1024))
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(map[string]any{
		"id": 1, "method": "initialize",
		"params": map[string]any{"clientInfo": map[string]string{"name": "gangline", "version": "1"}},
	}); err != nil {
		return nil, err
	}
	if _, err := limitsRPCResult(decoder, 1); err != nil {
		return nil, err
	}
	if err := encoder.Encode(map[string]any{"method": "initialized"}); err != nil {
		return nil, err
	}
	if err := encoder.Encode(map[string]any{"id": 2, "method": "account/rateLimits/read"}); err != nil {
		return nil, err
	}
	payload, err := limitsRPCResult(decoder, 2)
	if err != nil {
		return nil, err
	}
	return parseAccountLimits(payload)
}

func limitsRPCResult(decoder *json.Decoder, want int) (json.RawMessage, error) {
	for {
		var message struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := decoder.Decode(&message); err != nil {
			return nil, fmt.Errorf("read native limits response: %w", err)
		}
		if message.Method != "" {
			if message.ID != nil {
				return nil, fmt.Errorf("native limits query requires client action %q; gang will not answer it", message.Method)
			}
			continue
		}
		if message.ID == nil || *message.ID != want {
			return nil, fmt.Errorf("native limits query returned an unexpected response id")
		}
		if message.Error != nil {
			return nil, fmt.Errorf("native limits query: %s", message.Error.Message)
		}
		if len(message.Result) == 0 || string(message.Result) == "null" {
			return nil, fmt.Errorf("native limits query returned no result")
		}
		return message.Result, nil
	}
}

type accountLimitWindow struct {
	Used  *float64 `json:"usedPercent"`
	Reset *int64   `json:"resetsAt"`
}

type accountLimitBucket struct {
	ID        string              `json:"limitId"`
	Primary   *accountLimitWindow `json:"primary"`
	Secondary *accountLimitWindow `json:"secondary"`
}

func parseAccountLimits(payload []byte) ([]LimitWindow, error) {
	var response struct {
		Limits  *accountLimitBucket           `json:"rateLimits"`
		Buckets map[string]accountLimitBucket `json:"rateLimitsByLimitId"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode native account limits: %w", err)
	}
	buckets := response.Buckets
	if buckets == nil && response.Limits != nil {
		buckets = map[string]accountLimitBucket{response.Limits.ID: *response.Limits}
	}
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var limits []LimitWindow
	for _, key := range keys {
		bucket := buckets[key]
		for i, window := range []*accountLimitWindow{bucket.Primary, bucket.Secondary} {
			if window == nil {
				continue
			}
			label := "primary"
			if i == 1 {
				label = "secondary"
			}
			if key != "" {
				label = key + "/" + label
			}
			if window.Used == nil || window.Reset == nil || *window.Used < 0 || *window.Reset <= 0 {
				return nil, fmt.Errorf("native account limits carry an incomplete or invalid %s window", label)
			}
			limits = append(limits, LimitWindow{Label: label, UsedPercent: *window.Used, ResetAt: *window.Reset})
		}
	}
	if len(limits) == 0 {
		return nil, fmt.Errorf("native account exposes no provider-limit windows")
	}
	return limits, nil
}
