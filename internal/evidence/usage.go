package evidence

import (
	"encoding/json"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Usage imports one retained phase summary and its generation metadata export.
// It does not fetch prices, account information, or provider responses.
type Usage struct {
	Summary         string `json:"summary_artifact"`
	Charges         string `json:"charges_artifact"`
	RepairOf        string `json:"repair_of,omitempty"`
	RequestedModel  string `json:"requested_model,omitempty"`
	RequestedEffort string `json:"requested_effort,omitempty"`
}

type UsageResult struct {
	ActualUSD        *string        `json:"actual_usd"`
	KnownSubtotalUSD *string        `json:"known_subtotal_usd"`
	Generations      int            `json:"generations"`
	Attempts         []UsageAttempt `json:"attempts"`
}
type UsageAttempt struct {
	Record          string       `json:"record"`
	Session         string       `json:"session"`
	Phase           string       `json:"phase"`
	Started         string       `json:"started"`
	Completed       string       `json:"completed"`
	RepairOf        string       `json:"repair_of,omitempty"`
	RequestedModel  string       `json:"requested_model"`
	RequestedEffort string       `json:"requested_effort"`
	ReportedModel   string       `json:"reported_native_model"`
	ReportedEffort  string       `json:"reported_native_effort"`
	Assistance      string       `json:"frontier_assistance"`
	OwnerEdits      *int         `json:"owner_code_edits"`
	Repairs         *int         `json:"repairs"`
	ActualUSD       *string      `json:"actual_usd"`
	Missing         []string     `json:"missing_generations"`
	Charges         []Generation `json:"generations"`
	NativeUsage     *TokenUsage  `json:"native_cumulative_usage,omitempty"`
	PhaseDelta      *TokenCounts `json:"native_phase_delta,omitempty"`
}

type Generation struct {
	ID       string `json:"id"`
	Created  string `json:"created_at"`
	Model    string `json:"model"`
	Provider string `json:"provider_name"`
	USD      string `json:"actual_usd"`
}
type TokenCounts struct {
	Input       *int64 `json:"input_tokens,omitempty"`
	CachedInput *int64 `json:"cached_input_tokens,omitempty"`
	CacheWrite  *int64 `json:"cache_write_input_tokens,omitempty"`
	Output      *int64 `json:"output_tokens,omitempty"`
	Reasoning   *int64 `json:"reasoning_output_tokens,omitempty"`
	Total       *int64 `json:"total_tokens,omitempty"`
}
type TokenUsage struct {
	Total *TokenCounts `json:"total_token_usage,omitempty"`
	Last  *TokenCounts `json:"last_token_usage,omitempty"`
}

// These are deliberate projections. Commands, prompts and unknown fields are
// neither imported into the result nor quoted by decoder diagnostics.
type phaseSummary struct {
	Hitch   string `json:"hitch_id"`
	Session struct {
		ID string `json:"id"`
	} `json:"session"`
	Context struct {
		Model  string `json:"model"`
		Effort string `json:"effort"`
	} `json:"context"`
	ReportedModel  string `json:"requested_native_model"`
	ReportedEffort string `json:"requested_native_effort"`
	Boundaries     []struct {
		UTC  string `json:"utc"`
		Type string `json:"type"`
	} `json:"boundaries"`
	IDs            []string        `json:"generation_ids"`
	OwnerEdits     *int            `json:"owner_code_edits"`
	Repairs        *int            `json:"repairs"`
	AssistedBefore *bool           `json:"frontier_assistance_before_first_result"`
	Assistance     json.RawMessage `json:"frontier_assistance"`
	Usage          *TokenUsage     `json:"usage"`
	Cumulative     *TokenUsage     `json:"cumulative_usage"`
	Delta          *TokenCounts    `json:"phase_usage_delta"`
}
type chargeExport struct {
	Session  string          `json:"native_session"`
	Phase    string          `json:"phase"`
	Complete bool            `json:"complete"`
	Rows     []chargeRow     `json:"rows"`
	Failure  json.RawMessage `json:"failure"`
}
type chargeRow struct {
	Requested string  `json:"requested_id"`
	HTTP      int     `json:"http_status"`
	ID        string  `json:"id"`
	Created   string  `json:"created_at"`
	Model     string  `json:"model"`
	Provider  string  `json:"provider_name"`
	USD       *string `json:"total_cost_usd"`
}

var amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})(\.[0-9]{1,18})?$`)
var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,199}$`)

func money(s string) (*big.Rat, bool) {
	if !amountPattern.MatchString(s) {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}
func decimal(r *big.Rat) string {
	s := strings.TrimRight(strings.TrimRight(r.FloatString(18), "0"), ".")
	if s == "" {
		return "0"
	}
	return s
}
func reported(s string) string {
	if s == "" {
		return "unavailable"
	}
	return s
}
func (v *verifier) usage(records []Record, hitch string) {
	u := &UsageResult{Attempts: []UsageAttempt{}}
	v.result.Usage = u
	v.result.Coverage["sdlc_through_ci"] = "not_exercised"
	v.result.Coverage["native_turn_identity"] = "unavailable"
	v.result.Coverage["usage_provenance"] = "retained_local_metadata; not_provider_authenticated"
	v.result.Coverage["assignment_binding"] = "caller_declared"
	v.result.Coverage["billing_account"] = "unavailable"
	v.result.Coverage["estimated_cost"] = "not_supported"
	v.result.Coverage["assistance_cost"] = "unavailable"
	seen := map[string]Generation{}
	attempts := map[string]string{}
	byRecord := map[string]UsageAttempt{}
	total := new(big.Rat)
	session := ""
	for _, record := range records {
		if record.Kind != "usage" {
			continue
		}
		id, spec := record.ID, record.Usage
		issuesBefore := len(v.result.Issues)
		b, ok := v.artifact(spec.Summary)
		if !ok {
			continue
		}
		var s phaseSummary
		if json.Unmarshal(b, &s) != nil {
			v.issue(id, "fail", "invalid usage summary metadata")
			continue
		}
		if !identifier.MatchString(s.Session.ID) || (s.Hitch != "" && s.Hitch != hitch) || len(s.Boundaries) != 2 || s.Boundaries[0].Type != "task_started" || s.Boundaries[1].Type != "task_complete" {
			v.issue(id, "fail", "usage session or phase boundary invalid")
			continue
		}
		start, e1 := time.Parse(time.RFC3339Nano, s.Boundaries[0].UTC)
		end, e2 := time.Parse(time.RFC3339Nano, s.Boundaries[1].UTC)
		if e1 != nil || e2 != nil || !end.After(start) || (session != "" && session != s.Session.ID) {
			v.issue(id, "fail", "usage phases must name one session with valid boundaries")
			continue
		}
		session = s.Session.ID
		a := UsageAttempt{Record: id, Session: session, Started: s.Boundaries[0].UTC, Completed: s.Boundaries[1].UTC, RepairOf: spec.RepairOf, RequestedModel: reported(spec.RequestedModel), RequestedEffort: reported(spec.RequestedEffort), ReportedModel: reported(s.Context.Model), ReportedEffort: reported(s.Context.Effort), OwnerEdits: s.OwnerEdits, Repairs: s.Repairs, Assistance: "unavailable", Missing: []string{}, Charges: []Generation{}, NativeUsage: s.Usage, PhaseDelta: s.Delta}
		if s.ReportedModel != "" {
			a.ReportedModel = s.ReportedModel
		}
		if s.ReportedEffort != "" {
			a.ReportedEffort = s.ReportedEffort
		}
		if s.Cumulative != nil {
			a.NativeUsage = s.Cumulative
		}
		valid := true
		counts := []*TokenCounts{a.PhaseDelta}
		if a.NativeUsage != nil {
			counts = append(counts, a.NativeUsage.Total, a.NativeUsage.Last)
		}
		for _, c := range counts {
			if c == nil {
				continue
			}
			for _, n := range []*int64{c.Input, c.CachedInput, c.CacheWrite, c.Output, c.Reasoning, c.Total} {
				valid = valid && (n == nil || *n >= 0)
			}
		}
		for _, value := range []string{a.RequestedModel, a.RequestedEffort, a.ReportedModel, a.ReportedEffort} {
			valid = valid && modelName.MatchString(value)
		}
		if !valid || (s.OwnerEdits != nil && *s.OwnerEdits < 0) || (s.Repairs != nil && *s.Repairs < 0) {
			v.issue(id, "fail", "invalid usage model or assistance metadata")
			continue
		}
		if s.AssistedBefore != nil {
			a.Assistance = "none_reported_before_first_result"
			if *s.AssistedBefore {
				a.Assistance = "reported"
			}
		}
		if len(s.Assistance) != 0 && string(s.Assistance) != "null" {
			var assistance string
			if json.Unmarshal(s.Assistance, &assistance) != nil {
				v.issue(id, "fail", "invalid assistance metadata")
				continue
			}
			if assistance != "" {
				a.Assistance = "reported"
			}
		}
		expected := map[string]bool{}
		for _, generation := range s.IDs {
			if !identifier.MatchString(generation) {
				valid = false
			}
			expected[generation] = true
		}
		if !valid {
			v.issue(id, "fail", "invalid generation identity")
			continue
		}
		if len(expected) == 0 {
			v.issue(id, "incomplete", "expected generation coverage unavailable")
		}
		b, ok = v.artifact(spec.Charges)
		if !ok {
			for generation := range expected {
				a.Missing = append(a.Missing, generation)
			}
			sort.Strings(a.Missing)
			byRecord[id] = a
			u.Attempts = append(u.Attempts, a)
			continue
		}
		var charges chargeExport
		if json.Unmarshal(b, &charges) != nil || charges.Session != session || !identifier.MatchString(charges.Phase) {
			v.issue(id, "fail", "invalid charge export identity")
			continue
		}
		a.Phase = charges.Phase
		key := session + "/" + a.Started + "/" + a.Completed
		if _, exists := attempts[key]; exists {
			v.issue(id, "fail", "duplicate phase imported under another record")
			continue
		}
		attempts[key] = id
		if !charges.Complete || (len(charges.Failure) > 0 && string(charges.Failure) != "null") {
			v.issue(id, "incomplete", "provider charge export incomplete")
		}
		local := map[string]Generation{}
		rows := map[string]chargeRow{}
		partial := new(big.Rat)
		for _, row := range charges.Rows {
			if !expected[row.Requested] || (row.ID != "" && row.ID != row.Requested) {
				v.issue(id, "fail", "charge generation does not match phase")
				continue
			}
			if previous, exists := rows[row.Requested]; exists {
				if !reflect.DeepEqual(previous, row) {
					v.issue(id, "fail", "conflicting generation metadata")
				}
				continue
			}
			rows[row.Requested] = row
			if row.HTTP != 200 || row.USD == nil || row.ID == "" {
				continue
			}
			value, good := money(*row.USD)
			created, err := time.Parse(time.RFC3339Nano, row.Created)
			if !good || err != nil || created.Before(start) || created.After(end) || !modelName.MatchString(row.Model) || !modelName.MatchString(row.Provider) {
				v.issue(id, "fail", "invalid charge amount or generation metadata")
				continue
			}
			g := Generation{row.ID, row.Created, row.Model, row.Provider, decimal(value)}
			local[g.ID] = g
			if previous, exists := seen[g.ID]; exists {
				reason := "generation assigned to more than one phase"
				if !reflect.DeepEqual(previous, g) {
					reason = "conflicting generation metadata"
				}
				v.issue(id, "fail", reason)
				continue
			}
			seen[g.ID] = g
			total.Add(total, value)
			partial.Add(partial, value)
			a.Charges = append(a.Charges, g)
		}
		for generation := range expected {
			if _, exists := local[generation]; !exists {
				a.Missing = append(a.Missing, generation)
			}
		}
		sort.Strings(a.Missing)
		if len(a.Missing) > 0 {
			v.issue(id, "incomplete", "generation charges missing")
		} else if len(local) > 0 && len(v.result.Issues) == issuesBefore {
			amount := decimal(partial)
			a.ActualUSD = &amount
		}
		byRecord[id] = a
		u.Attempts = append(u.Attempts, a)
	}
	for _, a := range u.Attempts {
		if a.Repairs != nil && *a.Repairs > 0 && a.RepairOf == "" {
			v.issue(a.Record, "incomplete", "repair parent missing")
		}
		if a.RepairOf != "" {
			parent, exists := byRecord[a.RepairOf]
			parentEnd, _ := time.Parse(time.RFC3339Nano, parent.Completed)
			start, _ := time.Parse(time.RFC3339Nano, a.Started)
			if !exists || a.RepairOf == a.Record || parent.Session != a.Session || parentEnd.After(start) {
				v.issue(a.Record, "fail", "repair parent does not precede this phase")
			}
		}
	}
	if len(u.Attempts) == 0 {
		v.issue("", "incomplete", "usage attempts missing")
	}
	u.Generations = len(seen)
	if len(seen) > 0 {
		amount := decimal(total)
		u.KnownSubtotalUSD = &amount
		v.result.Coverage["cost"] = "partial_provider_reported_actual"
		if v.result.Status == "pass" {
			u.ActualUSD = &amount
			v.result.Coverage["cost"] = "provider_reported_actual_for_declared_generations"
		}
	}
	if v.result.Status == "fail" {
		u.ActualUSD, u.KnownSubtotalUSD = nil, nil
		v.result.Coverage["cost"] = "conflicting_or_invalid_metadata"
		for i := range u.Attempts {
			u.Attempts[i].ActualUSD = nil
		}
	}
}
