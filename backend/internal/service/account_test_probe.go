package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AccountTestProbeExtraKey stores the latest manual account connectivity probe
// in accounts.extra. It is deliberately separate from account status and
// schedulability: a transient provider quota failure should be visible without
// silently removing an account from the scheduler.
const AccountTestProbeExtraKey = "account_test_probe"

var ErrAccountTestProbeIdentityChanged = errors.New("account test probe identity changed")

// AccountTestProbeRepository provides an identity-checked write path for the
// manual probe snapshot. It is intentionally optional so read-only/lightweight
// AccountRepository implementations do not acquire a new mutation contract.
type AccountTestProbeRepository interface {
	UpdateAccountTestProbe(ctx context.Context, expected *Account, snapshot map[string]any) error
}

// AccountTestProbeCredentialIdentity removes short-lived OAuth material while
// retaining the provider/account/routing configuration that determines what a
// manual probe actually tested. Routine access-token rotation should not erase
// an otherwise valid manual result.
func AccountTestProbeCredentialIdentity(credentials map[string]any) map[string]any {
	identity := make(map[string]any, len(credentials))
	for key, value := range credentials {
		switch key {
		case "access_token", "id_token", "expires_at", "expires_in", "token_type":
			continue
		default:
			identity[key] = value
		}
	}
	return identity
}

func accountTestProbeSyncIdentity(account *Account) map[string]any {
	if account == nil {
		return nil
	}
	identity := map[string]any{
		"platform":    account.Platform,
		"type":        account.Type,
		"proxy_id":    nil,
		"credentials": AccountTestProbeCredentialIdentity(account.Credentials),
	}
	if account.ProxyID != nil {
		identity["proxy_id"] = *account.ProxyID
	}
	return identity
}

const (
	AccountTestProbeStatusSuccess = "success"
	AccountTestProbeStatusFailed  = "failed"

	AccountTestProbeReasonOK               = "ok"
	AccountTestProbeReasonQuotaExhausted   = "quota_exhausted"
	AccountTestProbeReasonAuthFailed       = "authentication_failed"
	AccountTestProbeReasonModelUnsupported = "model_unsupported"
	AccountTestProbeReasonRequestFailed    = "request_failed"
)

var accountTestProbeHTTPStatusPattern = regexp.MustCompile(`(?i)(?:api\s+(?:returned|返回)|http)\s*:?\s*([45][0-9]{2})`)

// accountTestProbeSnapshot returns a JSON-safe, credential-free snapshot for
// the admin account table. Keep only a bounded model name and a coarse reason;
// raw upstream bodies can contain provider metadata and must remain in logs.
func accountTestProbeSnapshot(modelID string, testErr error, checkedAt time.Time) map[string]any {
	snapshot := map[string]any{
		"status":     AccountTestProbeStatusSuccess,
		"checked_at": checkedAt.UTC().Format(time.RFC3339),
		"model":      truncateString(strings.TrimSpace(modelID), 128),
		"reason":     AccountTestProbeReasonOK,
	}
	if testErr == nil {
		return snapshot
	}

	snapshot["status"] = AccountTestProbeStatusFailed
	statusCode, reason := classifyAccountTestProbeError(testErr)
	if statusCode > 0 {
		snapshot["http_status"] = statusCode
	}
	snapshot["reason"] = reason
	return snapshot
}

func classifyAccountTestProbeError(testErr error) (int, string) {
	if testErr == nil {
		return 0, AccountTestProbeReasonOK
	}
	var httpErr *accountTestProbeHTTPError
	if errors.As(testErr, &httpErr) {
		return httpErr.StatusCode, httpErr.Reason
	}
	message := strings.ToLower(testErr.Error())
	statusCode := 0
	if match := accountTestProbeHTTPStatusPattern.FindStringSubmatch(testErr.Error()); len(match) == 2 {
		statusCode, _ = strconv.Atoi(match[1])
	}

	if statusCode == 429 || strings.Contains(message, "resource_exhausted") ||
		strings.Contains(message, "resource has been exhausted") ||
		strings.Contains(message, "resource exhausted") || strings.Contains(message, "rate limit") ||
		strings.Contains(message, "rate_limited") || strings.Contains(message, "限流") ||
		strings.Contains(message, "quota") || strings.Contains(message, "credit") {
		return statusCode, AccountTestProbeReasonQuotaExhausted
	}
	if strings.Contains(message, AccountTestProbeReasonModelUnsupported) ||
		strings.Contains(message, "unsupported model") || strings.Contains(message, "model not found") ||
		strings.Contains(message, "model_not_found") || strings.Contains(message, "model not supported") ||
		strings.Contains(message, "not in whitelist") {
		return statusCode, AccountTestProbeReasonModelUnsupported
	}
	if statusCode == 401 || statusCode == 403 || strings.Contains(message, "access_token") ||
		strings.Contains(message, "unauthor") || strings.Contains(message, "credential") {
		return statusCode, AccountTestProbeReasonAuthFailed
	}
	return statusCode, AccountTestProbeReasonRequestFailed
}

// accountTestProbeHTTPError keeps provider-specific response bodies out of
// SSE errors, logs, and the persisted probe snapshot while retaining the
// machine-readable classification needed by the operator UI.
type accountTestProbeHTTPError struct {
	StatusCode int
	Reason     string
	Model      string
	Prefix     string
}

func (e *accountTestProbeHTTPError) Error() string {
	if e == nil {
		return "account test request failed"
	}
	model := sanitizeAccountTestModelID(e.Model)
	prefix := strings.TrimSpace(e.Prefix)
	if prefix == "" {
		prefix = "API"
	}
	if model == "" {
		return fmt.Sprintf("%s returned %d (%s)", prefix, e.StatusCode, e.Reason)
	}
	return fmt.Sprintf("%s returned %d for model %q (%s)", prefix, e.StatusCode, model, e.Reason)
}

func sanitizeAccountTestModelID(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	modelID = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, modelID)
	return truncateString(modelID, 128)
}

func classifyAccountTestProbeHTTPBody(statusCode int, body []byte) string {
	message := strings.ToLower(string(body))
	if statusCode == 429 || strings.Contains(message, "resource_exhausted") ||
		strings.Contains(message, "resource exhausted") || strings.Contains(message, "rate limit") ||
		strings.Contains(message, "rate_limited") || strings.Contains(message, "quota") ||
		strings.Contains(message, "credit") || strings.Contains(message, "model_capacity_exhausted") {
		return AccountTestProbeReasonQuotaExhausted
	}
	if (statusCode == 400 || statusCode == 404 || statusCode == 422) &&
		(strings.Contains(message, "unsupported model") || strings.Contains(message, "model not found") ||
			strings.Contains(message, "model_not_found") || strings.Contains(message, "model not supported") ||
			strings.Contains(message, "does not support the requested model") || strings.Contains(message, "invalid model")) {
		return AccountTestProbeReasonModelUnsupported
	}
	if statusCode == 401 || statusCode == 403 || strings.Contains(message, "unauthenticated") ||
		strings.Contains(message, "invalid_grant") || strings.Contains(message, "invalid access token") {
		return AccountTestProbeReasonAuthFailed
	}
	return AccountTestProbeReasonRequestFailed
}

func newAccountTestProbeHTTPError(statusCode int, body []byte, modelID string) error {
	return newAccountTestProbeHTTPErrorWithPrefix(statusCode, body, modelID, "API")
}

func newAccountTestProbeHTTPErrorWithPrefix(statusCode int, body []byte, modelID, prefix string) error {
	return &accountTestProbeHTTPError{
		StatusCode: statusCode,
		Reason:     classifyAccountTestProbeHTTPBody(statusCode, body),
		Model:      modelID,
		Prefix:     prefix,
	}
}

// sanitizeAccountTestErrorMessage is a final defense for non-HTTP errors that
// may wrap provider text. Known HTTP prefixes are reduced to status/reason and
// credential-like fields are removed before an SSE event or log is emitted.
func sanitizeAccountTestErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "account test request failed"
	}
	if match := accountTestProbeHTTPStatusPattern.FindStringSubmatch(message); len(match) == 2 {
		status := match[1]
		lower := strings.ToLower(message)
		if strings.Contains(lower, "api returned") || strings.Contains(lower, "api 返回") ||
			strings.Contains(lower, "responses api returned") {
			return "API returned " + status
		}
	}
	for _, key := range []string{"access_token", "refresh_token", "authorization", "api_key", "password", "client_secret"} {
		message = redactAccountTestErrorField(message, key)
	}
	return truncateString(message, 512)
}

func redactAccountTestErrorField(message, key string) string {
	lower := strings.ToLower(message)
	for {
		idx := strings.Index(lower, key)
		if idx < 0 {
			return message
		}
		valueStart := idx + len(key)
		for valueStart < len(message) && (message[valueStart] == ' ' || message[valueStart] == ':' || message[valueStart] == '=') {
			valueStart++
		}
		if strings.HasPrefix(strings.ToLower(message[valueStart:]), "bearer ") {
			valueStart += len("bearer ")
		}
		valueEnd := valueStart
		for valueEnd < len(message) && message[valueEnd] != ' ' && message[valueEnd] != ',' && message[valueEnd] != '}' && message[valueEnd] != '"' {
			valueEnd++
		}
		if valueStart == idx+len(key) && valueStart < len(message) && message[valueStart] != ' ' && message[valueStart] != ':' && message[valueStart] != '=' {
			// The key was embedded in ordinary prose; redact only the token-like
			// suffix rather than deleting the surrounding message.
			valueEnd = valueStart
			for valueEnd < len(message) && message[valueEnd] != ' ' && message[valueEnd] != ',' && message[valueEnd] != '}' && message[valueEnd] != '"' {
				valueEnd++
			}
		}
		message = message[:idx] + "[redacted]" + message[valueEnd:]
		lower = strings.ToLower(message)
	}
}

func (s *AccountTestService) persistAccountTestProbe(ctx context.Context, account *Account, modelID string, testErr error) {
	if account == nil {
		return
	}
	snapshot := accountTestProbeSnapshot(modelID, testErr, time.Now())
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra[AccountTestProbeExtraKey] = snapshot
	if s == nil || s.accountRepo == nil {
		return
	}
	probeRepo, ok := s.accountRepo.(AccountTestProbeRepository)
	if !ok {
		return
	}
	baseCtx := context.Background()
	if ctx != nil {
		baseCtx = context.WithoutCancel(ctx)
	}
	updateCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()
	if err := probeRepo.UpdateAccountTestProbe(updateCtx, account, snapshot); err != nil && !errors.Is(err, ErrAccountTestProbeIdentityChanged) {
		log.Printf("account test probe persistence failed: account=%d error=%v", account.ID, err)
	}
}
