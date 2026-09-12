package types

import (
	"encoding/json"
	"testing"
)

// The CLI reports a retry as a system message whose payload sits at the top
// level, not inside "data". Those fields used to be dropped, so a consumer
// could see that a retry happened but not why: this is what a provider outage
// (an expired plan returned as HTTP 429, say) looks like on the wire.
func TestSystemMessageKeepsTopLevelFields(t *testing.T) {
	raw := `{"type":"system","subtype":"api_retry","attempt":3,"max_retries":10,"retry_delay_ms":2115,"error_status":429,"error":"rate_limit","session_id":"abc"}`

	var msg SystemMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Subtype != "api_retry" {
		t.Fatalf("subtype = %q", msg.Subtype)
	}
	for key, want := range map[string]interface{}{
		"attempt": float64(3), "max_retries": float64(10), "error_status": float64(429), "error": "rate_limit",
	} {
		if got := msg.Data[key]; got != want {
			t.Errorf("Data[%q] = %v, want %v", key, got, want)
		}
	}
}

// An explicit "data" object still wins, and is not clobbered by a top-level
// field of the same name.
func TestSystemMessageExplicitDataWins(t *testing.T) {
	raw := `{"type":"system","subtype":"init","data":{"tools":["Read"],"attempt":"from-data"},"attempt":"top-level","cwd":"/tmp"}`

	var msg SystemMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := msg.Data["attempt"]; got != "from-data" {
		t.Errorf(`Data["attempt"] = %v, want "from-data" (explicit data must win)`, got)
	}
	if got := msg.Data["cwd"]; got != "/tmp" {
		t.Errorf(`Data["cwd"] = %v, want "/tmp" (unknown top-level fields are merged in)`, got)
	}
	if tools, ok := msg.Data["tools"].([]interface{}); !ok || len(tools) != 1 {
		t.Errorf(`Data["tools"] = %v, want the original list`, msg.Data["tools"])
	}
}

// control_request/control_response share the type; their known fields must
// still land in their own struct fields, not in Data.
func TestSystemMessageControlFieldsUnchanged(t *testing.T) {
	raw := `{"type":"control_request","request_id":"r1","request":{"tool":"Bash"}}`

	var msg SystemMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.RequestID != "r1" || msg.Request["tool"] != "Bash" {
		t.Fatalf("control fields lost: %+v", msg)
	}
	if _, leaked := msg.Data["request"]; leaked {
		t.Error("known fields must not be duplicated into Data")
	}
}
