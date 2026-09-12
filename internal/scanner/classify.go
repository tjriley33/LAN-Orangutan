package scanner

import (
	"strings"

	"github.com/291-Group/LAN-Orangutan/internal/types"
)

// Device type labels produced by Classify. They are stable strings so stored
// records, the API and the UI can all rely on them. TypeUnknown is the empty
// string so an unclassified device carries no type rather than a placeholder.
const (
	TypePhone       = "Phone"
	TypeComputer    = "Computer"
	TypePrinter     = "Printer"
	TypeTV          = "TV"
	TypeMediaPlayer = "Media Player"
	TypeRouter      = "Router"
	TypeServer      = "Server"
	TypeIoT         = "IoT"
	TypeConsole     = "Game Console"
	TypeUnknown     = ""
)

// KnownTypes lists every type Classify can return, so the UI can offer them as
// suggestions when the user sets a device's type by hand.
var KnownTypes = []string{
	TypeComputer, TypeConsole, TypeIoT, TypeMediaPlayer, TypePhone,
	TypePrinter, TypeRouter, TypeServer, TypeTV,
}

// InferredType returns the type detected for a device, ignoring any override
// the user set: the type recorded at scan time or, for a record that predates
// classification, one inferred now from its vendor and scanned hostname. Pass
// the resolved vendor (see ResolveVendor). Port evidence is not available here,
// so the backfill is the default path, not the opt-in probe.
func InferredType(d *types.Device, vendor string) string {
	if d.Type != "" {
		return d.Type
	}
	return Classify(vendor, d.Hostname, nil)
}

// ResolveType returns the type to show for a device: the user's CustomType when
// one is set, otherwise the inferred type. Clearing the override therefore falls
// straight back to detection instead of leaving the type blank.
func ResolveType(d *types.Device, vendor string) string {
	if d.CustomType != "" {
		return d.CustomType
	}
	return InferredType(d, vendor)
}

// Classify infers a device's type from the signals a scan gathers: its vendor
// (from the MAC OUI), its hostname, and, when service detection is enabled, the
// open ports found. It returns one of the Type* labels, or TypeUnknown when
// nothing matches.
//
// Signals are weighed by how specific they are. A hostname or an open service
// port identifies a device far more reliably than its vendor, which is often
// shared across very different products: Apple makes phones, laptops and TVs
// under one OUI. So hostname wins first, then ports, and vendor is the broad
// fallback that still covers a device with no useful name.
func Classify(vendor, hostname string, openPorts []int) string {
	if t := classifyByHostname(hostname); t != TypeUnknown {
		return t
	}
	// Definitive ports identify one specific kind of hardware (a printer, an
	// iPhone) and outrank the broad vendor guess.
	if t := classifyByPorts(openPorts, definitivePorts); t != TypeUnknown {
		return t
	}
	if t := classifyByVendor(vendor); t != TypeUnknown {
		return t
	}
	// Indicative ports point to a role a device is playing (a media server, a
	// NAS service) rather than the hardware itself, so they only classify when
	// vendor and hostname gave nothing. A Mac running Plex stays a Computer.
	return classifyByPorts(openPorts, indicativePorts)
}

// rule pairs a set of lower-case substrings with the type they indicate. The
// first rule whose substring is contained in the (lower-cased) input wins, so
// more specific rules are listed before broader ones.
type rule struct {
	needles []string
	typ     string
}

func matchRules(s string, rules []rule) string {
	s = strings.ToLower(s)
	if s == "" {
		return TypeUnknown
	}
	for _, r := range rules {
		for _, n := range r.needles {
			if strings.Contains(s, n) {
				return r.typ
			}
		}
	}
	return TypeUnknown
}

// hostnameRules read the friendly name a device advertises. These are the most
// reliable signal, so a printer that calls itself "laserjet" is a printer even
// if its vendor string is generic.
var hostnameRules = []rule{
	// Consoles first: "switch" is too generic alone, so match the full name.
	{[]string{"xbox", "playstation", "ps4", "ps5", "nintendo"}, TypeConsole},
	// Media players before TVs, since an Apple TV or Fire TV is a player.
	{[]string{"appletv", "apple-tv", "firetv", "fire-tv", "roku", "chromecast", "shield", "plex", "kodi", "sonos"}, TypeMediaPlayer},
	{[]string{"printer", "officejet", "laserjet", "deskjet", "envy", "mfc", "brother", "epson", "canon", "-hp"}, TypePrinter},
	{[]string{"iphone", "ipad", "android", "galaxy", "pixel", "oneplus", "-phone"}, TypePhone},
	{[]string{"macbook", "imac", "-mac", "desktop-", "laptop", "thinkpad", "surface", "-pc", "windows"}, TypeComputer},
	{[]string{"bravia", "vizio", "webos", "samsungtv", "-tv", "smarttv"}, TypeTV},
	{[]string{"router", "gateway", "unifi", "ubnt", "openwrt", "dd-wrt", "eero", "orbi", "-ap", "accesspoint", "gl-inet", "glinet"}, TypeRouter},
	{[]string{"nas", "synology", "diskstation", "qnap", "truenas", "raspberrypi", "raspberry", "pihole", "pi-hole", "proxmox", "server", "ubuntu", "debian"}, TypeServer},
	{[]string{"esp32", "esp8266", "esp-", "shelly", "sonoff", "tasmota", "tuya", "nest", "ring", "wyze", "-hue", "smartthings", "echo", "alexa"}, TypeIoT},
}

func classifyByHostname(hostname string) string {
	return matchRules(hostname, hostnameRules)
}

// definitivePorts point at exactly one kind of hardware: only a printer speaks
// JetDirect or IPP, only an iPhone answers the iOS lockdown port. These outrank
// the vendor guess. Generic ports like 80, 443 and 22 are deliberately absent:
// almost anything serves those, so they identify nothing on their own.
var definitivePorts = map[int]string{
	9100:  TypePrinter, // raw JetDirect printing
	631:   TypePrinter, // IPP
	515:   TypePrinter, // LPD
	62078: TypePhone,   // iOS lockdown / iTunes sync
}

// indicativePorts suggest a service a device is running rather than what the
// device is. Plex or a Synology admin port can appear on a NAS, a server or a
// repurposed PC, so these only classify a device whose vendor and hostname gave
// nothing, and never override a known vendor.
var indicativePorts = map[int]string{
	32400: TypeMediaPlayer, // Plex
	8096:  TypeMediaPlayer, // Jellyfin
	5000:  TypeServer,      // Synology DSM (http), also many dev servers
	5001:  TypeServer,      // Synology DSM (https)
	548:   TypeServer,      // AFP, typically a NAS
}

func classifyByPorts(openPorts []int, table map[int]string) string {
	for _, p := range openPorts {
		if t, ok := table[p]; ok {
			return t
		}
	}
	return TypeUnknown
}

// vendorRules read the manufacturer from the MAC OUI. This is the broadest
// signal and the one most devices have, but it is also the least precise, since
// large makers ship many kinds of hardware. Single-purpose makers (a printer or
// switch vendor) classify cleanly; multi-purpose makers get a best guess that
// the user can override with a label.
var vendorRules = []rule{
	{[]string{"hewlett packard ent", "aruba"}, TypeRouter}, // HPE networking, before generic HP
	{[]string{"canon", "epson", "brother", "lexmark", "xerox", "kyocera", "ricoh", "hewlett-packard", "hp inc"}, TypePrinter},
	{[]string{"ubiquiti", "tp-link", "netgear", "d-link", "mikrotik", "cisco", "juniper", "ruckus", "zyxel", "meraki", "eero", "gl technologies", "gl-inet"}, TypeRouter},
	{[]string{"synology", "qnap", "raspberry pi", "western digital", "supermicro"}, TypeServer},
	{[]string{"roku", "sonos", "google, inc"}, TypeMediaPlayer},
	{[]string{"vizio", "tcl", "hisense", "skyworth"}, TypeTV},
	{[]string{"espressif", "tuya", "shelly", "sonoff", "signify", "philips lighting", "wyze", "ecobee", "particle", "espressif inc"}, TypeIoT},
	{[]string{"nintendo", "sony interactive"}, TypeConsole},
	{[]string{"intel", "dell", "lenovo", "asustek", "micro-star", "gigabyte", "framework"}, TypeComputer},
	{[]string{"apple"}, TypeComputer}, // Apple: many products; Computer is the safe default without a hostname
}

func classifyByVendor(vendor string) string {
	return matchRules(vendor, vendorRules)
}
