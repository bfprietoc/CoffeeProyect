package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Provider identifies the payment provider stored on an order.
const ProviderWompi = "wompi"

// CheckoutParams are the hidden-input values the frontend renders in the
// Wompi Web Checkout form. They are computed server-side (the integrity
// secret must never reach the browser) and returned to the client verbatim.
type CheckoutParams struct {
	PublicKey     string `json:"publicKey"`
	Reference     string `json:"reference"`
	AmountInCents int    `json:"amountInCents"`
	Currency      string `json:"currency"`
	Signature     string `json:"signature"`
	RedirectURL   string `json:"redirectUrl"`
	CustomerEmail string `json:"customerEmail"`
}

// IntegritySignature computes the Web Checkout integrity hash:
//
//	SHA256(reference + amount_in_cents + currency + integritySecret)
//
// See https://docs.wompi.co/docs/colombia/widget-checkout-web/
// ("Paso 3: Genera una firma de integridad"). Hex-encoded lowercase to match
// Wompi's examples (PHP/Python/Ruby all produce lowercase hex).
func IntegritySignature(reference string, amountInCents int, currency string, integritySecret string) string {
	chain := fmt.Sprintf("%s%d%s%s", reference, amountInCents, currency, integritySecret)
	sum := sha256.Sum256([]byte(chain))
	return hex.EncodeToString(sum[:])
}

// WebhookEvent is a parsed Wompi webhook payload (transaction.updated).
type WebhookEvent struct {
	Event       string          `json:"event"`
	Environment string          `json:"environment"`
	Timestamp   int64           `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
	Signature   struct {
		Properties []string `json:"properties"`
		Checksum   string   `json:"checksum"`
	} `json:"signature"`
}

// TransactionData holds the subset of transaction.updated data needed to
// drive the payment state machine.
type TransactionData struct {
	ID            string `json:"id"`
	Reference     string `json:"reference"`
	Status        string `json:"status"`
	AmountInCents int    `json:"amount_in_cents"`
	Currency      string `json:"currency"`
	PaymentMethod string `json:"payment_method_type"`
}

// VerifyAndParseEvent validates the event checksum and returns the parsed
// event. The checksum is SHA256 hex of:
//
//	(concat of data[property] for each property in signature.properties, in order)
//	+ timestamp + eventSecret
//
// See https://docs.wompi.co/docs/colombia/eventos/ (Paso 1–4). Signature
// verification is case-insensitive on the hex checksum because Wompi's
// examples show uppercase while the PHP/Python examples produce lowercase.
// Properties are taken from the event itself (they may vary per event).
func VerifyAndParseEvent(rawBody []byte, eventSecret string) (*WebhookEvent, error) {
	var evt WebhookEvent
	if err := json.Unmarshal(rawBody, &evt); err != nil {
		return nil, fmt.Errorf("parse webhook body: %w", err)
	}
	if evt.Event == "" {
		return nil, fmt.Errorf("missing event type")
	}
	if len(evt.Signature.Properties) == 0 {
		return nil, fmt.Errorf("missing signature properties")
	}

	// Extract values from the data object using dotted property paths
	// (e.g. "transaction.id" → data.transaction.id).
	var data map[string]any
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		return nil, fmt.Errorf("parse webhook data: %w", err)
	}
	chain := ""
	for _, prop := range evt.Signature.Properties {
		v, ok := nestedValue(data, prop)
		if !ok {
			return nil, fmt.Errorf("missing property %q in event data", prop)
		}
		chain += v
	}
	chain += fmt.Sprintf("%d", evt.Timestamp)
	chain += eventSecret
	sum := sha256.Sum256([]byte(chain))
	computed := hex.EncodeToString(sum[:])

	provided := strings.ToUpper(strings.TrimSpace(evt.Signature.Checksum))
	if !strings.EqualFold(computed, provided) {
		return nil, fmt.Errorf("invalid webhook signature")
	}
	return &evt, nil
}

// nestedValue extracts data["a"]["b"] along dotted path "a.b". Values are
// stringified: strings as-is, numbers/booleans via fmt, others as JSON.
func nestedValue(data map[string]any, path string) (string, bool) {
	parts := strings.Split(path, ".")
	var cur any = data
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[p]
		if !ok {
			return "", false
		}
	}
	switch v := cur.(type) {
	case string:
		return v, true
	case float64:
		// JSON numbers: render as int when whole (amount_in_cents is an int)
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%v", v), true
	case bool:
		return fmt.Sprintf("%t", v), true
	case nil:
		return "", false
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// TransactionFromEvent extracts the transaction from a transaction.updated
// event. ok is false when the event carries no transaction (or the event is
// not transaction.updated — callers decide what to do with that).
func TransactionFromEvent(evt *WebhookEvent) (*TransactionData, bool) {
	if evt.Event != "transaction.updated" {
		return nil, false
	}
	var data struct {
		Transaction TransactionData `json:"transaction"`
	}
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		return nil, false
	}
	if data.Transaction.ID == "" || data.Transaction.Reference == "" {
		return nil, false
	}
	return &data.Transaction, true
}

// FinalStatus maps a Wompi transaction status to our PaymentStatus.
// APPROVED → paid; everything terminal (DECLINED/ERROR/VOIDED) → failed;
// anything else (PENDING) → pending (caller decides whether to act).
func FinalStatus(wompiStatus string) (status string, terminal bool) {
	switch strings.ToUpper(strings.TrimSpace(wompiStatus)) {
	case "APPROVED":
		return "paid", true
	case "DECLINED", "ERROR", "VOIDED":
		return "failed", true
	default:
		return "pending", false
	}
}
