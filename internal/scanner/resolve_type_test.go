package scanner

import (
	"testing"

	"github.com/291-Group/LAN-Orangutan/internal/types"
)

func TestResolveTypePrefersCustomType(t *testing.T) {
	cases := []struct {
		name         string
		device       types.Device
		vendor       string
		want         string
		wantInferred string
	}{
		{"custom beats stored type", types.Device{Type: TypePrinter, CustomType: "3D Printer"}, "", "3D Printer", TypePrinter},
		{"custom beats classification", types.Device{Hostname: "office-laserjet", CustomType: "Label Maker"}, "", "Label Maker", TypePrinter},
		{"stored type with no custom", types.Device{Type: TypeRouter}, "", TypeRouter, TypeRouter},
		{"no custom falls back to hostname", types.Device{Hostname: "living-room-roku"}, "", TypeMediaPlayer, TypeMediaPlayer},
		{"no custom falls back to vendor", types.Device{}, "Espressif Inc.", TypeIoT, TypeIoT},
		{"nothing known stays unknown", types.Device{}, "", TypeUnknown, TypeUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveType(&tc.device, tc.vendor); got != tc.want {
				t.Errorf("ResolveType() = %q, want %q", got, tc.want)
			}
			if got := InferredType(&tc.device, tc.vendor); got != tc.wantInferred {
				t.Errorf("InferredType() = %q, want %q", got, tc.wantInferred)
			}
		})
	}
}

func TestKnownTypesAreAllReal(t *testing.T) {
	seen := map[string]bool{}
	for _, typ := range KnownTypes {
		if typ == TypeUnknown {
			t.Error("KnownTypes should not offer the empty unknown type")
		}
		if seen[typ] {
			t.Errorf("KnownTypes lists %q twice", typ)
		}
		seen[typ] = true
	}
}
