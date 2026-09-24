package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLimitsQueryProtocolCreatesNoThreadOrTurn(t *testing.T) {
	input := `{"id":1,"result":{}}
{"method":"account/rateLimits/updated","params":{"rateLimits":{"primary":{"usedPercent":99,"resetsAt":1}}}}
{"id":2,"result":{"rateLimits":{"limitId":"codex","primary":{"usedPercent":23,"resetsAt":1800000000}}}}
`
	var output bytes.Buffer
	limits, err := queryCodexLimits(strings.NewReader(input), &output)
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 1 || limits[0].UsedPercent != 23 || limits[0].Label != "codex/primary" {
		t.Fatalf("limits = %#v", limits)
	}
	var methods []string
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var request struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, request.Method)
	}
	if strings.Join(methods, ",") != "initialize,initialized,account/rateLimits/read" {
		t.Fatalf("native requests = %v", methods)
	}
}

func TestLimitsQueryRejectsUnobservedValues(t *testing.T) {
	for name, payload := range map[string]string{
		"absent":                 `{}`,
		"empty":                  `{"rateLimits":{}}`,
		"missing_usage":          `{"rateLimits":{"primary":{"resetsAt":1800000000}}}`,
		"missing_reset":          `{"rateLimits":{"primary":{"usedPercent":0}}}`,
		"null_usage":             `{"rateLimits":{"primary":{"usedPercent":null,"resetsAt":1800000000}}}`,
		"negative_usage":         `{"rateLimits":{"primary":{"usedPercent":-1,"resetsAt":1800000000}}}`,
		"invalid_reset":          `{"rateLimits":{"primary":{"usedPercent":0,"resetsAt":0}}}`,
		"empty_explicit_buckets": `{"rateLimits":{"primary":{"usedPercent":55,"resetsAt":1800000000}},"rateLimitsByLimitId":{}}`,
		"malformed":              `!`,
	} {
		t.Run(name, func(t *testing.T) {
			limits, err := parseAccountLimits([]byte(payload))
			if err == nil || len(limits) != 0 {
				t.Fatalf("limits = %#v, error = %v", limits, err)
			}
		})
	}
}

func TestLimitsQueryUsesAllBucketsInStableOrder(t *testing.T) {
	payload := `{"rateLimits":{"primary":{"usedPercent":99,"resetsAt":1}},"rateLimitsByLimitId":{
 "other":{"primary":{"usedPercent":12,"resetsAt":1800000000}},
 "codex":{"primary":{"usedPercent":0,"resetsAt":1800000000},"secondary":{"usedPercent":34,"resetsAt":1800000001}}
 }}`
	limits, err := parseAccountLimits([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 3 || limits[0].Label != "codex/primary" || limits[0].UsedPercent != 0 || limits[1].Label != "codex/secondary" || limits[2].Label != "other/primary" {
		t.Fatalf("limits = %#v", limits)
	}
}

func TestLimitsQueryRefusesServerRequestsAndProtocolErrors(t *testing.T) {
	for name, input := range map[string]string{
		"approval":    `{"id":7,"method":"item/commandExecution/requestApproval","params":{}}`,
		"auth":        `{"id":1,"result":{}}` + "\n" + `{"id":2,"error":{"message":"not logged in"}}`,
		"wrong_id":    `{"id":3,"result":{}}`,
		"null_result": `{"id":1,"result":null}`,
		"closed":      ``,
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			limits, err := queryCodexLimits(strings.NewReader(input), &output)
			if err == nil || len(limits) != 0 {
				t.Fatalf("limits = %#v, error = %v", limits, err)
			}
			if strings.Contains(output.String(), `"result"`) || strings.Contains(output.String(), `"error"`) {
				t.Fatalf("answered server request: %s", output.String())
			}
		})
	}
}

func TestLimitsQuerySubprocessClosesAndReaps(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "native-cli")
	done := filepath.Join(root, "closed")
	script := `#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
[ "$*" = 'app-server --listen stdio://' ] || exit 80
IFS= read -r request || exit 81
case "$request" in *'"method":"initialize"'*) ;; *) exit 82;; esac
printf '%s\n' '{"id":1,"result":{}}'
IFS= read -r request || exit 83
[ "$request" = '{"method":"initialized"}' ] || exit 84
IFS= read -r request || exit 85
case "$request" in *'"method":"account/rateLimits/read"'*) ;; *) exit 86;; esac
printf '%s\n' '{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":0,"resetsAt":1800000000}}}}'
if IFS= read -r request; then exit 87; fi
: > "$LIMITS_CLOSED"
`
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	collar := Collar{Name: "fixture", Launch: Launch{Command: executable, Env: map[string]string{"LIMITS_CLOSED": done}}, Primitives: Primitives{LimitsQuery: &Invocation{Name: "codex-app-server-limits"}}}
	limits, err := QueryProviderLimits(context.Background(), collar)
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 1 || limits[0].UsedPercent != 0 {
		t.Fatalf("limits = %#v", limits)
	}
	if _, err := os.Stat(done); err != nil {
		t.Fatalf("child did not finish before return: %v", err)
	}
}

func TestLimitsQueryCancelledBeforeLaunch(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "started")
	executable := filepath.Join(root, "provider")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf started > \"$START_MARKER\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	collar := Collar{Name: "fixture", Launch: Launch{Command: executable, Env: map[string]string{"START_MARKER": marker}}, Primitives: Primitives{LimitsQuery: &Invocation{Name: "codex-app-server-limits"}}}
	if limits, err := QueryProviderLimits(ctx, collar); !errors.Is(err, context.Canceled) || len(limits) != 0 {
		t.Fatalf("limits = %#v, error = %v", limits, err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled provider started: %v", err)
	}
}
