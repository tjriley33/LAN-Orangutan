package storage

import (
	"path/filepath"
	"testing"

	"github.com/291-Group/LAN-Orangutan/internal/types"
)

const (
	macA = "AA:AA:AA:AA:AA:AA"
	macB = "BB:BB:BB:BB:BB:BB"
	ipX  = "192.168.68.10"
	ipY  = "192.168.68.11"
	ipZ  = "192.168.68.12"
)

func merge(t *testing.T, s *Storage, devs ...types.Device) {
	t.Helper()
	if err := s.MergeDevices(devs); err != nil {
		t.Fatal(err)
	}
}

func label(t *testing.T, s *Storage, ip, l, host string) {
	t.Helper()
	if err := s.EditDevice(ip, DeviceEdit{Label: &l, CustomHostname: &host}); err != nil {
		t.Fatal(err)
	}
}

// TestReassignedIPDoesNotInheritLabels is the DHCP case that used to mislabel
// devices: A goes offline, its IP is handed to B, and B must not show A's
// label. A's data must survive and return with A at whatever IP it gets.
func TestReassignedIPDoesNotInheritLabels(t *testing.T) {
	s := newTestStorage(t)
	merge(t, s, types.Device{IP: ipX, MAC: macA, Hostname: "a-scan"})
	label(t, s, ipX, "Laundry", "Water Monitor")

	// A is offline; B now answers at A's old address.
	merge(t, s, types.Device{IP: ipX, MAC: macB, Hostname: "b-scan"})

	b := s.GetDevice(ipX)
	if b == nil || b.MAC != macB {
		t.Fatalf("B should be at %s: %+v", ipX, b)
	}
	if b.Label != "" || b.CustomHostname != "" {
		t.Errorf("B inherited A's labels: %+v", b)
	}
	if b.Hostname != "b-scan" {
		t.Errorf("B should carry its own scanned hostname, got %q", b.Hostname)
	}

	// A comes back somewhere else.
	merge(t, s, types.Device{IP: ipZ, MAC: macA, Hostname: "a-scan"})
	a := s.GetDevice(ipZ)
	if a == nil || a.Label != "Laundry" || a.CustomHostname != "Water Monitor" {
		t.Fatalf("A's labels should return with it: %+v", a)
	}
	if len(a.AddressHistory) != 1 || a.AddressHistory[0].IP != ipX {
		t.Errorf("A's move from %s should be recorded: %+v", ipX, a.AddressHistory)
	}
	if len(s.parked) != 0 {
		t.Errorf("A should no longer be parked: %+v", s.parked)
	}
}

// TestSwappedIPsKeepTheirLabels: two devices trade addresses in one scan, in
// either order of discovery.
func TestSwappedIPsKeepTheirLabels(t *testing.T) {
	orders := map[string][]types.Device{
		"A first": {{IP: ipY, MAC: macA}, {IP: ipX, MAC: macB}},
		"B first": {{IP: ipX, MAC: macB}, {IP: ipY, MAC: macA}},
	}
	for name, scan := range orders {
		t.Run(name, func(t *testing.T) {
			s := newTestStorage(t)
			merge(t, s, types.Device{IP: ipX, MAC: macA}, types.Device{IP: ipY, MAC: macB})
			label(t, s, ipX, "Shed", "Device A")
			label(t, s, ipY, "Den", "Device B")

			merge(t, s, scan...)

			if d := s.GetDevice(ipY); d == nil || d.MAC != macA || d.CustomHostname != "Device A" || d.Label != "Shed" {
				t.Errorf("A at %s lost its labels: %+v", ipY, d)
			}
			if d := s.GetDevice(ipX); d == nil || d.MAC != macB || d.CustomHostname != "Device B" || d.Label != "Den" {
				t.Errorf("B at %s lost its labels: %+v", ipX, d)
			}
			if len(s.GetDevices()) != 2 || len(s.parked) != 0 {
				t.Errorf("expected 2 devices and nothing parked, got %d and %d", len(s.GetDevices()), len(s.parked))
			}
		})
	}
}

// TestParkedDeviceSurvivesRestart: the parked list is persisted, so a restart
// between losing the IP and coming back does not lose the labels.
func TestParkedDeviceSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	devs, state := filepath.Join(dir, "devices.json"), filepath.Join(dir, "state.json")
	s, err := New(devs, state)
	if err != nil {
		t.Fatal(err)
	}
	merge(t, s, types.Device{IP: ipX, MAC: macA})
	label(t, s, ipX, "Kitchen", "Ice Maker")
	merge(t, s, types.Device{IP: ipX, MAC: macB})

	s2, err := New(devs, state)
	if err != nil {
		t.Fatal(err)
	}
	merge(t, s2, types.Device{IP: ipZ, MAC: macA})
	if d := s2.GetDevice(ipZ); d == nil || d.CustomHostname != "Ice Maker" || d.Label != "Kitchen" {
		t.Errorf("parked labels lost across restart: %+v", d)
	}
}

// TestUncuratedDisplacedDeviceIsNotParked: a device with nothing the user set
// is just replaced, so rotating private MACs cannot pile up in the parked list.
func TestUncuratedDisplacedDeviceIsNotParked(t *testing.T) {
	s := newTestStorage(t)
	merge(t, s, types.Device{IP: ipX, MAC: macA})
	merge(t, s, types.Device{IP: ipX, MAC: macB})
	if len(s.parked) != 0 {
		t.Errorf("nothing should be parked: %+v", s.parked)
	}
	if d := s.GetDevice(ipX); d == nil || d.MAC != macB {
		t.Errorf("B should be at %s: %+v", ipX, d)
	}
}

// TestMACMatchIgnoresCase: one source reporting lower-case MACs must not split
// a device in two.
func TestMACMatchIgnoresCase(t *testing.T) {
	s := newTestStorage(t)
	merge(t, s, types.Device{IP: ipX, MAC: macA})
	label(t, s, ipX, "Office", "Closet Light")
	merge(t, s, types.Device{IP: ipX, MAC: "aa:aa:aa:aa:aa:aa"})
	if d := s.GetDevice(ipX); d == nil || d.CustomHostname != "Closet Light" {
		t.Errorf("a case-only MAC difference should be the same device: %+v", d)
	}
}
