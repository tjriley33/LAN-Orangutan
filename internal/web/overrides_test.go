package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/291-Group/LAN-Orangutan/internal/storage"
	"github.com/291-Group/LAN-Orangutan/internal/types"
)

func renderDashboard(t *testing.T, h *Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestDashboardShowsOverridesAndFallsBack renders a device with a custom hostname
// and type, then clears both and checks the row falls back to the scanned
// hostname and the inferred type rather than going blank.
func TestDashboardShowsOverridesAndFallsBack(t *testing.T) {
	h, _ := newTestHandler(t, "")
	ip := "192.168.68.62"
	// No stored type, so the dashboard has to infer one from the hostname.
	if err := h.store.MergeDevices([]types.Device{{IP: ip, Hostname: "garage-printer"}}); err != nil {
		t.Fatal(err)
	}

	name, deviceType := "Bambu P2S", "3D Printer"
	if err := h.store.EditDevice(ip, storage.DeviceEdit{CustomHostname: &name, CustomType: &deviceType}); err != nil {
		t.Fatal(err)
	}
	body := renderDashboard(t, h)
	for _, want := range []string{
		`data-hostname="bambu p2s"`,
		`data-scanned-hostname="garage-printer"`,
		`data-custom-hostname="Bambu P2S"`,
		`data-type="3d printer"`,
		`data-custom-type="3D Printer"`,
		`data-auto-type="Printer"`,
		`type-badge-custom`,
		`<option value="Game Console">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard with overrides is missing %s", want)
		}
	}

	empty := ""
	if err := h.store.EditDevice(ip, storage.DeviceEdit{CustomHostname: &empty, CustomType: &empty}); err != nil {
		t.Fatal(err)
	}
	body = renderDashboard(t, h)
	for _, want := range []string{
		`data-hostname="garage-printer"`,
		`data-custom-hostname=""`,
		`data-type="printer"`,
		`data-custom-type=""`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard after clearing is missing %s", want)
		}
	}
	if strings.Contains(body, "Bambu P2S") || strings.Contains(body, "type-badge-custom") {
		t.Error("cleared overrides still show on the dashboard")
	}
}
