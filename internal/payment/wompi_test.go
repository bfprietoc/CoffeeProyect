package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestIntegritySignature_matchesWompiExample(t *testing.T) {
	// Official example from https://docs.wompi.co/docs/colombia/widget-checkout-web/
	// reference: sk8-438k4-xmxm392-sn2m, amount: 2490000, COP,
	// integrity secret: prod_integrity_Z5mMke9x0k8gpErbDqwrJXMqsI6SFli6
	got := IntegritySignature("sk8-438k4-xmxm392-sn2m", 2490000, "COP", "prod_integrity_Z5mMke9x0k8gpErbDqwrJXMqsI6SFli6")
	want := "37c8407747e595535433ef8f6a811d853cd943046624a0ec04662b17bbf33bf5"
	if got != want {
		t.Errorf("integrity signature mismatch:\n got  %s\n want %s", got, want)
	}
}

// buildEvent constructs a transaction.updated webhook with a VALID checksum
// (mirrors the documented steps: properties values + timestamp + secret).
func buildEvent(t *testing.T, properties []string, data map[string]any, timestamp int64, secret string) []byte {
	t.Helper()
	chain := ""
	for _, prop := range properties {
		chain += readProp(data, prop)
	}
	chain += fmt.Sprintf("%d", timestamp)
	chain += secret
	sum := sha256.Sum256([]byte(chain))
	checksum := hex.EncodeToString(sum[:])

	evt := map[string]any{
		"event":       "transaction.updated",
		"environment": "test",
		"timestamp":   timestamp,
		"data":        data,
		"signature":   map[string]any{"properties": properties, "checksum": checksum},
	}
	raw, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return raw
}

// readProp reads a dotted path (e.g. "transaction.id") from the data map,
// stringifying values like the parser does.
func readProp(data map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var cur any = data
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[p]
		if !ok {
			return ""
		}
	}
	switch v := cur.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func TestVerifyAndParseEvent_valid(t *testing.T) {
	secret := "prod_events_testsecret123"
	data := map[string]any{
		"transaction": map[string]any{
			"id":                  "1234-1610641025-49201",
			"reference":           "order-abc-1705330245",
			"status":              "APPROVED",
			"amount_in_cents":     4490000,
			"currency":            "COP",
			"payment_method_type": "CARD",
		},
	}
	raw := buildEvent(t, []string{"transaction.id", "transaction.status", "transaction.amount_in_cents"}, data, 1530291411, secret)

	evt, err := VerifyAndParseEvent(raw, secret)
	if err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	tx, ok := TransactionFromEvent(evt)
	if !ok {
		t.Fatal("expected a transaction from transaction.updated event")
	}
	if tx.ID != "1234-1610641025-49201" {
		t.Errorf("wrong transaction id: %s", tx.ID)
	}
	if tx.Status != "APPROVED" {
		t.Errorf("wrong transaction status: %s", tx.Status)
	}
	if tx.Reference != "order-abc-1705330245" {
		t.Errorf("wrong transaction reference: %s", tx.Reference)
	}
}

func TestVerifyAndParseEvent_checksumMismatch(t *testing.T) {
	secret := "prod_events_testsecret123"
	data := map[string]any{
		"transaction": map[string]any{
			"id":     "tx-1",
			"status": "APPROVED",
		},
	}
	raw := buildEvent(t, []string{"transaction.id", "transaction.status"}, data, 1530291411, secret)
	// flip a character in the checksum
	var asMap map[string]any
	_ = json.Unmarshal(raw, &asMap)
	sig := asMap["signature"].(map[string]any)
	sig["checksum"] = "0000" + sig["checksum"].(string)[4:]
	broken, _ := json.Marshal(asMap)

	if _, err := VerifyAndParseEvent(broken, secret); err == nil {
		t.Fatal("expected checksum mismatch to be rejected")
	}
}

func TestVerifyAndParseEvent_wrongSecret(t *testing.T) {
	data := map[string]any{
		"transaction": map[string]any{"id": "tx-1", "status": "APPROVED"},
	}
	raw := buildEvent(t, []string{"transaction.id"}, data, 1530291411, "right-secret")
	if _, err := VerifyAndParseEvent(raw, "wrong-secret"); err == nil {
		t.Fatal("expected verification to fail with the wrong secret")
	}
}

func TestVerifyAndParseEvent_missingProperty(t *testing.T) {
	secret := "prod_events_testsecret123"
	data := map[string]any{
		"transaction": map[string]any{"id": "tx-1"}, // no status
	}
	raw := buildEvent(t, []string{"transaction.id", "transaction.status"}, data, 1530291411, secret)
	// the checksum is computed over "" for the missing prop, but verification
	// must fail (missing property) instead of validating a bogus chain.
	if _, err := VerifyAndParseEvent(raw, secret); err == nil {
		t.Fatal("expected missing property to be rejected")
	}
}

func TestTransactionFromEvent_wrongEvent(t *testing.T) {
	raw := []byte(`{"event":"nequi_token.updated","timestamp":1530291411,"signature":{"properties":["x"],"checksum":"abc"},"data":{}}`)
	evt := &WebhookEvent{Event: "nequi_token.updated"}
	if _, ok := TransactionFromEvent(evt); ok {
		t.Error("nequi_token.updated must not produce a transaction")
	}
	_ = raw // (parse path covered by VerifyAndParseEvent)
}

func TestFinalStatus(t *testing.T) {
	cases := []struct {
		wompi  string
		status string
		term   bool
	}{
		{"APPROVED", "paid", true},
		{"approved", "paid", true},
		{"DECLINED", "failed", true},
		{"ERROR", "failed", true},
		{"VOIDED", "failed", true},
		{"PENDING", "pending", false},
		{"WHATEVER", "pending", false},
	}
	for _, c := range cases {
		status, term := FinalStatus(c.wompi)
		if status != c.status || term != c.term {
			t.Errorf("FinalStatus(%q) = (%s, %v), want (%s, %v)", c.wompi, status, term, c.status, c.term)
		}
	}
}
