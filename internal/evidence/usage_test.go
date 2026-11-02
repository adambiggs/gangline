package evidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func usageFixture() *fixture {
	f := &fixture{m: Manifest{Version: 1, Scope: Scope{"host", "team", "hitch", "assignment"}}, files: map[string][]byte{}}
	f.artifact("summary", []byte(`{"hitch_id":"hitch","session":{"id":"session"},"context":{"model":"test/model","effort":"high"},"boundaries":[{"type":"task_started","utc":"2026-11-02T01:00:00Z"},{"type":"task_complete","utc":"2026-11-02T01:01:00Z"}],"generation_ids":["gen-a","gen-b"],"owner_code_edits":0,"frontier_assistance_before_first_result":false,"usage":{"total_token_usage":{"input_tokens":100}}}`))
	f.artifact("charges", []byte(`{"native_session":"session","phase":"first","complete":true,"rows":[{"requested_id":"gen-a","id":"gen-a","http_status":200,"created_at":"2026-11-02T01:00:01Z","model":"test/model","provider_name":"test-provider","total_cost_usd":"0.1"},{"requested_id":"gen-b","id":"gen-b","http_status":200,"created_at":"2026-11-02T01:00:02Z","model":"test/model","provider_name":"test-provider","total_cost_usd":"0.2"}],"total_cost_usd":"99999"}`))
	f.add("first", "usage", &Usage{Summary: "summary", Charges: "charges", RequestedModel: "test/model", RequestedEffort: "high"})
	return f
}
func usageResult(f *fixture) Result {
	return Verify(f.m, "assignment", "usage", func(path string, _ int64) ([]byte, error) { return f.files[path], nil })
}
func TestUsageSumsRowsExactlyAndKeepsCoverageSeparate(t *testing.T) {
	r := usageResult(usageFixture())
	if r.Status != "pass" || r.Usage.ActualUSD == nil || *r.Usage.ActualUSD != "0.3" || r.Usage.Generations != 2 {
		t.Fatalf("%+v", r)
	}
	a := r.Usage.Attempts[0]
	if a.Assistance != "none_reported_before_first_result" || a.OwnerEdits == nil || *a.OwnerEdits != 0 || a.NativeUsage == nil || *a.NativeUsage.Total.Input != 100 {
		t.Fatalf("attempt %+v", a)
	}
	if r.Coverage["backend_effort"] != "unavailable" || r.Coverage["sdlc_through_ci"] != "not_exercised" || r.Coverage["billing_account"] != "unavailable" || r.Coverage["assistance_cost"] != "unavailable" {
		t.Fatalf("coverage %+v", r.Coverage)
	}
}
func TestUsageDeduplicatesRepeatedRowsAndRecords(t *testing.T) {
	f := usageFixture()
	changeJSON(f, "charges", func(m map[string]any) { rows := m["rows"].([]any); m["rows"] = append(rows, rows[0]) })
	changeJSON(f, "summary", func(m map[string]any) { m["generation_ids"] = []string{"gen-a", "gen-b", "gen-a"} })
	f.m.Records = append(f.m.Records, *f.get("first"))
	r := usageResult(f)
	if r.Status != "pass" || *r.Usage.ActualUSD != "0.3" || r.Usage.Generations != 2 || len(r.Usage.Attempts) != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestUsageUnknownChargeIsNotZero(t *testing.T) {
	f := usageFixture()
	changeJSON(f, "charges", func(m map[string]any) { m["rows"].([]any)[1].(map[string]any)["total_cost_usd"] = nil })
	r := usageResult(f)
	if r.Status != "incomplete" || r.Usage.ActualUSD != nil || *r.Usage.KnownSubtotalUSD != "0.1" || !hasReason(r, "generation charges missing") || len(r.Usage.Attempts[0].Missing) != 1 {
		t.Fatalf("%+v", r)
	}
	changeJSON(f, "charges", func(m map[string]any) { m["rows"] = []any{} })
	r = usageResult(f)
	if r.Usage.KnownSubtotalUSD != nil || r.Usage.ActualUSD != nil || r.Coverage["cost"] != "unavailable" {
		t.Fatalf("%+v", r)
	}
}
func TestUsageRejectsConflictsAndUnlinkedRows(t *testing.T) {
	cases := []struct {
		name, reason string
		change       func(*fixture)
	}{
		{"changed amount", "conflicting generation metadata", func(f *fixture) {
			changeJSON(f, "charges", func(m map[string]any) {
				rows := m["rows"].([]any)
				copy := map[string]any{}
				for k, v := range rows[0].(map[string]any) {
					copy[k] = v
				}
				copy["total_cost_usd"] = "0.9"
				m["rows"] = append(rows, copy)
			})
		}},
		{"wrong requested id", "charge generation does not match phase", func(f *fixture) {
			changeJSON(f, "charges", func(m map[string]any) { m["rows"].([]any)[0].(map[string]any)["requested_id"] = "unknown" })
		}},
		{"wrong session", "invalid charge export identity", func(f *fixture) { changeJSON(f, "charges", func(m map[string]any) { m["native_session"] = "other" }) }},
		{"negative money", "invalid charge amount or generation metadata", func(f *fixture) {
			changeJSON(f, "charges", func(m map[string]any) { m["rows"].([]any)[0].(map[string]any)["total_cost_usd"] = "-0.1" })
		}},
		{"outside phase", "invalid charge amount or generation metadata", func(f *fixture) {
			changeJSON(f, "charges", func(m map[string]any) { m["rows"].([]any)[0].(map[string]any)["created_at"] = "2026-11-03T01:00:00Z" })
		}},
		{"other hitch", "usage session or phase boundary invalid", func(f *fixture) { changeJSON(f, "summary", func(m map[string]any) { m["hitch_id"] = "other" }) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := usageFixture()
			tt.change(f)
			r := usageResult(f)
			if r.Status != "fail" || !hasReason(r, tt.reason) || r.Usage.ActualUSD != nil || r.Usage.KnownSubtotalUSD != nil {
				t.Fatalf("%+v", r)
			}
		})
	}
}
func TestUsageRepairPhaseAndCrossPhaseDuplicates(t *testing.T) {
	f := usageFixture()
	f.artifact("repair-summary", []byte(`{"session":{"id":"session"},"requested_native_model":"test/model","requested_native_effort":"high","boundaries":[{"type":"task_started","utc":"2026-11-02T01:02:00Z"},{"type":"task_complete","utc":"2026-11-02T01:03:00Z"}],"generation_ids":["gen-c"],"repairs":1,"frontier_assistance":"review finding","owner_code_edits":0}`))
	f.artifact("repair-charges", []byte(`{"native_session":"session","phase":"guided_revision","complete":true,"rows":[{"requested_id":"gen-c","id":"gen-c","http_status":200,"created_at":"2026-11-02T01:02:01Z","model":"test/model","provider_name":"test-provider","total_cost_usd":"0.04"}]}`))
	f.add("repair", "usage", &Usage{Summary: "repair-summary", Charges: "repair-charges", RepairOf: "first"})
	r := usageResult(f)
	if r.Status != "pass" || *r.Usage.ActualUSD != "0.34" || len(r.Usage.Attempts) != 2 || r.Usage.Attempts[1].Assistance != "reported" || *r.Usage.Attempts[1].ActualUSD != "0.04" {
		t.Fatalf("%+v", r)
	}
	f.get("charges").Artifact.Status = "unavailable"
	r = usageResult(f)
	if r.Status != "incomplete" || !hasReason(r, "artifact unavailable") || hasReason(r, "repair parent does not precede this phase") || r.Usage.ActualUSD != nil || r.Usage.KnownSubtotalUSD == nil || *r.Usage.KnownSubtotalUSD != "0.04" || r.Usage.Attempts[1].ActualUSD == nil || *r.Usage.Attempts[1].ActualUSD != "0.04" {
		t.Fatalf("unavailable parent charges must retain the known repair subtotal: %+v", r)
	}
	f.get("charges").Artifact.Status = "available"
	f.get("repair").Usage.RepairOf = ""
	r = usageResult(f)
	if r.Status != "incomplete" || !hasReason(r, "repair parent missing") {
		t.Fatalf("%+v", r)
	}
	f.get("repair").Usage.RepairOf = "first"
	// A repeated generation cannot be attributed twice even if each export says complete.
	changeJSON(f, "repair-summary", func(m map[string]any) { m["generation_ids"] = []string{"gen-a"} })
	changeJSON(f, "repair-charges", func(m map[string]any) {
		row := m["rows"].([]any)[0].(map[string]any)
		row["id"] = "gen-a"
		row["requested_id"] = "gen-a"
	})
	r = usageResult(f)
	if r.Status != "fail" || !hasReason(r, "conflicting generation metadata") || r.Usage.ActualUSD != nil {
		t.Fatalf("%+v", r)
	}
}
func TestUsageProjectionDoesNotImportOrPrintCommandContent(t *testing.T) {
	const sentinel = "INERT_COMMAND_ARGUMENT_AND_UNKNOWN_FIELD_SENTINEL"
	f := usageFixture()
	changeJSON(f, "summary", func(m map[string]any) {
		m["commands"] = []any{map[string]any{"arguments": sentinel}}
		m["unknown"] = sentinel
		m["session"].(map[string]any)["prompt"] = sentinel
		m["frontier_assistance"] = sentinel
	})
	changeJSON(f, "charges", func(m map[string]any) {
		m["unknown"] = sentinel
		m["rows"].([]any)[0].(map[string]any)["prompt"] = sentinel
	})
	r := usageResult(f)
	b, _ := json.Marshal(r)
	if r.Status != "pass" || *r.Usage.ActualUSD != "0.3" || strings.Contains(string(b), sentinel) || strings.Contains(string(b), "commands") {
		t.Fatalf("projection failed: status=%s", r.Status)
	}
	// Known-field type errors must not quote the rejected object or nested data.
	changeJSON(f, "summary", func(m map[string]any) { m["generation_ids"] = map[string]any{"commands": sentinel} })
	r = usageResult(f)
	b, _ = json.Marshal(r)
	if r.Status != "fail" || !hasReason(r, "invalid usage summary metadata") || strings.Contains(string(b), sentinel) {
		t.Fatalf("unsafe error projection: status=%s", r.Status)
	}
}
