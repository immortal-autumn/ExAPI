package privatecutover

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectedAndParseConfirmation(t *testing.T) {
	expected := ExpectedConfirmation(42)
	require.Equal(t, "DROP-SAAS-DATA-KEEP-USER-42", expected)
	operatorID, err := ParseConfirmation(expected)
	require.NoError(t, err)
	require.Equal(t, int64(42), operatorID)
}

func TestParseConfirmationRejectsNearMisses(t *testing.T) {
	for _, value := range []string{
		"",
		"DROP-SAAS-DATA-KEEP-USER-",
		"DROP-SAAS-DATA-KEEP-USER-0",
		"DROP-SAAS-DATA-KEEP-USER-1-extra",
		"KEEP-USER-1",
	} {
		_, err := ParseConfirmation(value)
		require.Error(t, err, value)
	}
}

func TestSignReportRequiresKeyAndIncludesIntegrityFields(t *testing.T) {
	_, err := SignReport(MigrationReport{OperatorID: 42}, []byte("short"))
	require.Error(t, err)

	signed, err := SignReport(MigrationReport{SchemaVersion: 1, OperatorID: 42}, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	require.Contains(t, string(signed), `"report_sha256"`)
	require.Contains(t, string(signed), `"report_hmac_sha256"`)
}
