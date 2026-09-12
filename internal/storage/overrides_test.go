package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/291-Group/LAN-Orangutan/internal/types"
)

const overrideIP = "192.168.68.62"
const overrideMAC = "EC:B5:0A:29:01:C5"

func setOverrides(t *testing.T, s *Storage, ip, hostname, deviceType string) {
	t.Helper()
	if err := s.EditDevice(ip, DeviceEdit{CustomHostname: &hostname, CustomType: &deviceType}); err != nil {
		t.Fatalf("EditDevice: %v", err)
	}
}

// TestOverridesSurviveEveryKindOfScan: a custom hostname and type belong to the
// user, so no scan path may overwrite them, while the scanned values they sit
// alongside keep refreshing underneath.
func TestOverridesSurviveEveryKindOfScan(t *testing.T) {
	s := newTestStorage(t)
	if err := s.MergeDevices([]types.Device{{IP: overrideIP, MAC: overrideMAC, Hostname: "scanned-name", Type: "IoT"}}); err != nil {
		t.Fatal(err)
	}
	setOverrides(t, s, overrideIP, "Bambu P2S", "3D Printer")

	if err := s.MergeDevices([]types.Device{{IP: overrideIP, MAC: overrideMAC, Hostname: "renamed-by-dns", Type: "Printer"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeSupplemental([]types.Device{{IP: overrideIP, Hostname: "mdns-name", Type: "Media Player"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeIPv6Neighbors([]types.Device{{IP: "2001:db8::62", MAC: overrideMAC, Hostname: "v6-name"}}); err != nil {
		t.Fatal(err)
	}

	d := s.GetDevice(overrideIP)
	if d == nil {
		t.Fatal("device disappeared")
	}
	if d.CustomHostname != "Bambu P2S" || d.CustomType != "3D Printer" {
		t.Errorf("overrides lost across scans: hostname %q, type %q", d.CustomHostname, d.CustomType)
	}
	if d.Hostname != "renamed-by-dns" || d.Type != "Printer" {
		t.Errorf("scanned values should keep refreshing underneath: hostname %q, type %q", d.Hostname, d.Type)
	}
	if got := d.DisplayHostname(); got != "Bambu P2S" {
		t.Errorf("DisplayHostname() = %q, want the custom name", got)
	}
}

func TestOverridesSurviveReload(t *testing.T) {
	dir := t.TempDir()
	devicesFile, stateFile := filepath.Join(dir, "devices.json"), filepath.Join(dir, "state.json")

	s, err := New(devicesFile, stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MergeDevices([]types.Device{{IP: overrideIP, Hostname: "scanned-name"}}); err != nil {
		t.Fatal(err)
	}
	setOverrides(t, s, overrideIP, "Bambu P2S", "3D Printer")

	reloaded, err := New(devicesFile, stateFile)
	if err != nil {
		t.Fatal(err)
	}
	d := reloaded.GetDevice(overrideIP)
	if d == nil || d.CustomHostname != "Bambu P2S" || d.CustomType != "3D Printer" {
		t.Errorf("overrides not persisted: %+v", d)
	}
}

// TestClearingOverridesFallsBackToScanned covers the clear path, including a
// whitespace-only value, which must not become an invisible override.
func TestClearingOverridesFallsBackToScanned(t *testing.T) {
	s := newTestStorage(t)
	if err := s.MergeDevices([]types.Device{{IP: overrideIP, Hostname: "scanned-name", Type: "IoT"}}); err != nil {
		t.Fatal(err)
	}
	setOverrides(t, s, overrideIP, "Bambu P2S", "3D Printer")
	setOverrides(t, s, overrideIP, "", "   ")

	d := s.GetDevice(overrideIP)
	if d.CustomHostname != "" || d.CustomType != "" {
		t.Errorf("overrides not cleared: hostname %q, type %q", d.CustomHostname, d.CustomType)
	}
	if got := d.DisplayHostname(); got != "scanned-name" {
		t.Errorf("DisplayHostname() = %q, want the scanned name back", got)
	}
	if d.Type != "IoT" {
		t.Errorf("clearing the custom type should not touch the inferred one, got %q", d.Type)
	}
}

func TestEditLeavesOmittedFieldsAlone(t *testing.T) {
	s := newTestStorage(t)
	if err := s.MergeDevices([]types.Device{{IP: overrideIP, Hostname: "scanned-name"}}); err != nil {
		t.Fatal(err)
	}
	setOverrides(t, s, overrideIP, "Bambu P2S", "3D Printer")

	// An older caller that only knows about label, notes and group.
	if err := s.UpdateDeviceFields(overrideIP, strptr("Garage"), nil, nil); err != nil {
		t.Fatal(err)
	}
	// Clearing only the type.
	if err := s.EditDevice(overrideIP, DeviceEdit{CustomType: strptr("")}); err != nil {
		t.Fatal(err)
	}

	d := s.GetDevice(overrideIP)
	if d.Label != "Garage" || d.CustomHostname != "Bambu P2S" || d.CustomType != "" {
		t.Errorf("unexpected device after partial edits: %+v", d)
	}
}

func TestOverridesFollowDeviceToNewIP(t *testing.T) {
	s := newTestStorage(t)
	if err := s.MergeDevices([]types.Device{{IP: overrideIP, MAC: overrideMAC, Hostname: "scanned-name"}}); err != nil {
		t.Fatal(err)
	}
	setOverrides(t, s, overrideIP, "Bambu P2S", "3D Printer")

	if err := s.MergeDevices([]types.Device{{IP: "192.168.68.63", MAC: overrideMAC, Hostname: "scanned-name"}}); err != nil {
		t.Fatal(err)
	}

	d := s.GetDevice("192.168.68.63")
	if d == nil || d.CustomHostname != "Bambu P2S" || d.CustomType != "3D Printer" {
		t.Errorf("overrides should move with the device's MAC: %+v", d)
	}
}

func TestUpdateDeviceKeepsOverrides(t *testing.T) {
	s := newTestStorage(t)
	if err := s.MergeDevices([]types.Device{{IP: overrideIP, Hostname: "scanned-name"}}); err != nil {
		t.Fatal(err)
	}
	setOverrides(t, s, overrideIP, "Bambu P2S", "3D Printer")

	if err := s.UpdateDevice(&types.Device{IP: overrideIP, Hostname: "replacement"}); err != nil {
		t.Fatal(err)
	}

	d := s.GetDevice(overrideIP)
	if d.CustomHostname != "Bambu P2S" || d.CustomType != "3D Printer" {
		t.Errorf("UpdateDevice dropped the overrides: %+v", d)
	}
}

// TestPruneKeepsIPv6DeviceWithOnlyOverrides: a custom name or type is curation
// just like a label, so the IPv6 cleanup must not delete the device.
func TestPruneKeepsIPv6DeviceWithOnlyOverrides(t *testing.T) {
	dir := t.TempDir()
	devicesFile := filepath.Join(dir, "devices.json")
	data := `{"fe80::abcd": {"ip": "fe80::abcd", "custom_hostname": "Doorbell"}}`
	if err := os.WriteFile(devicesFile, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	s, err := New(devicesFile, filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.GetDevice("fe80::abcd") == nil {
		t.Error("a device with a custom hostname should survive the IPv6 cleanup")
	}
}
