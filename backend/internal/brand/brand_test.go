package brand

import "testing"

func TestDefaults(t *testing.T) {
	if ProductName != "ExAPI" {
		t.Fatalf("ProductName=%q, want ExAPI", ProductName)
	}
	if ProductDescription != "ExAPI - AI API Gateway Platform" {
		t.Fatalf("ProductDescription=%q", ProductDescription)
	}
	if DefaultAdminEmail != "admin@exapi.local" {
		t.Fatalf("DefaultAdminEmail=%q", DefaultAdminEmail)
	}
	if DefaultSiteTitle != "ExAPI - AI API Gateway" {
		t.Fatalf("DefaultSiteTitle=%q", DefaultSiteTitle)
	}
}
