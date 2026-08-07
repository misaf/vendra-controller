package urls

import "testing"

func TestPlatformReportsTheLoopbackDashboard(t *testing.T) {
	values := Platform("example.com")

	if got, want := values["dashboard"], "http://127.0.0.1:8080/dashboard/"; got != want {
		t.Fatalf("dashboard URL = %q, want %q", got, want)
	}
}
