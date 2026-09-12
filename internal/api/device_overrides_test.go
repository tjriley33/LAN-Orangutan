package api

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/291-Group/LAN-Orangutan/internal/config"
	"github.com/291-Group/LAN-Orangutan/internal/storage"
	"github.com/291-Group/LAN-Orangutan/internal/types"
)

func newTestAPI(t *testing.T) (*Handler, *storage.Storage) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.New(filepath.Join(dir, "devices.json"), filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	return NewHandler(store, config.Default()), store
}

func postDevice(t *testing.T, h *Handler, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/device", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/device %s: status %d, body %s", body, rec.Code, rec.Body.String())
	}
}

func TestPostDeviceSetsAndClearsOverrides(t *testing.T) {
	h, store := newTestAPI(t)
	ip := "192.168.68.62"
	if err := store.MergeDevices([]types.Device{{IP: ip, Hostname: "scanned-name", Type: "IoT"}}); err != nil {
		t.Fatal(err)
	}

	postDevice(t, h, `{"ip":"192.168.68.62","custom_hostname":"Bambu P2S","custom_type":"3D Printer"}`)
	d := store.GetDevice(ip)
	if d.CustomHostname != "Bambu P2S" || d.CustomType != "3D Printer" {
		t.Fatalf("overrides not set: %+v", d)
	}

	// A request without the override fields, as an older client sends, leaves
	// them alone.
	postDevice(t, h, `{"ip":"192.168.68.62","label":"Garage"}`)
	d = store.GetDevice(ip)
	if d.Label != "Garage" || d.CustomHostname != "Bambu P2S" || d.CustomType != "3D Printer" {
		t.Fatalf("label-only update disturbed the overrides: %+v", d)
	}

	postDevice(t, h, `{"ip":"192.168.68.62","custom_hostname":"","custom_type":""}`)
	d = store.GetDevice(ip)
	if d.CustomHostname != "" || d.CustomType != "" || d.Label != "Garage" {
		t.Fatalf("overrides not cleared cleanly: %+v", d)
	}
	if d.Hostname != "scanned-name" || d.Type != "IoT" {
		t.Fatalf("scanned values should be untouched: %+v", d)
	}
}

func TestCSVExportShowsOverrides(t *testing.T) {
	h, store := newTestAPI(t)
	ip := "192.168.68.62"
	if err := store.MergeDevices([]types.Device{{IP: ip, Hostname: "scanned-name", Type: "IoT"}}); err != nil {
		t.Fatal(err)
	}
	name, deviceType := "Bambu P2S", "3D Printer"
	if err := store.EditDevice(ip, storage.DeviceEdit{CustomHostname: &name, CustomType: &deviceType}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/devices?format=csv", nil))
	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("CSV: %v, rows %v", err, rows)
	}

	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	row := rows[1]
	if got := row[col["Hostname"]]; got != "Bambu P2S" {
		t.Errorf("Hostname column = %q, want the custom name", got)
	}
	if got := row[col["Scanned Hostname"]]; got != "scanned-name" {
		t.Errorf("Scanned Hostname column = %q", got)
	}
	if got := row[col["Type"]]; got != "3D Printer" {
		t.Errorf("Type column = %q, want the custom type", got)
	}
}
