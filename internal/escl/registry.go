package escl

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// Device is a scanner we know about, seen on the network now or earlier.
type Device struct {
	GUID     string    `json:"guid"`
	Name     string    `json:"name"`
	Model    string    `json:"model"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Scheme   string    `json:"scheme"` // http or https
	Root     string    `json:"root"`   // path of the eSCL service, default "eSCL"
	LastSeen time.Time `json:"lastSeen"`
	Online   bool      `json:"online"`
	Manual   bool      `json:"manual"` // added by address, not found by mDNS
}

// BaseURL is the root of the scanner's eSCL service.
func (d Device) BaseURL() string {
	base := d.Scheme + "://" + net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
	if d.Root != "" {
		base += "/" + d.Root
	}
	return base
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// GUID is a stable id of a scanner: the UUID it announces (most eSCL scanners do), else a UUID derived from data that
// does not change between searches: the mDNS host name and the model. No IP address (DHCP can change it) and no
// service name (mDNS appends " (2)" on name conflicts).
func GUID(announced, host, model string) string {
	a := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(announced)), "urn:uuid:")))
	if uuidRe.MatchString(a) {
		return a
	}
	sum := sha1.Sum([]byte("mcprint|" + strings.ToLower(host) + "|" + model))
	sum[6] = sum[6]&0x0f | 0x50 // version 5 (name-based)
	sum[8] = sum[8]&0x3f | 0x80 // RFC 4122 variant
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func txtMap(txt []string) map[string]string {
	m := map[string]string{}
	for _, t := range txt {
		if k, v, ok := strings.Cut(t, "="); ok {
			m[strings.ToLower(k)] = v
		}
	}
	return m
}

// deviceFromEntry builds a Device from an mDNS answer, or returns false if it is unusable.
func deviceFromEntry(e *zeroconf.ServiceEntry) (Device, bool) {
	txt := txtMap(e.Text)
	host := ""
	switch {
	case len(e.AddrIPv4) > 0:
		host = e.AddrIPv4[0].String()
	case len(e.AddrIPv6) > 0 && !e.AddrIPv6[0].IsLinkLocalUnicast():
		host = e.AddrIPv6[0].String()
	default:
		host = strings.TrimSuffix(e.HostName, ".")
	}
	if host == "" || !IsLAN(host) {
		return Device{}, false
	}
	scheme := "http"
	if strings.Contains(e.Service, "_uscans") {
		scheme = "https"
	}
	root := strings.Trim(txt["rs"], "/")
	if _, ok := txt["rs"]; !ok {
		root = "eSCL"
	}
	name := txt["ty"]
	if name == "" {
		name = e.Instance
	}
	return Device{
		GUID:   GUID(txt["uuid"], strings.TrimSuffix(e.HostName, "."), txt["ty"]),
		Name:   name,
		Model:  txt["ty"],
		Host:   host,
		Port:   e.Port,
		Scheme: scheme,
		Root:   root,
		Online: true,
	}, true
}

type entry struct {
	Device
	misses int
}

// Registry keeps the scanners found so far, searches for more in the background and remembers them between runs.
type Registry struct {
	path     string
	onChange func()
	// OnSweep, if set, is called with true when a search starts and false when it ends.
	OnSweep func(searching bool)
	kick    chan struct{}

	mu   sync.Mutex
	devs map[string]*entry
}

// NewRegistry loads the saved devices from path (they start offline until they are seen again). onChange is called
// whenever the list changes.
func NewRegistry(path string, onChange func()) *Registry {
	r := &Registry{path: path, onChange: onChange, kick: make(chan struct{}, 1), devs: map[string]*entry{}}
	if data, err := os.ReadFile(path); err == nil {
		var saved []Device
		if json.Unmarshal(data, &saved) == nil {
			for _, d := range saved {
				d.Online = false
				if d.GUID != "" && d.Host != "" {
					r.devs[d.GUID] = &entry{Device: d}
				}
			}
		}
	}
	return r
}

// NewMockRegistry is a registry with made-up devices that is never searched, saved or changed by the network (demo mode).
func NewMockRegistry(devices []Device) *Registry {
	r := &Registry{kick: make(chan struct{}, 1), devs: map[string]*entry{}}
	for _, d := range devices {
		r.devs[d.GUID] = &entry{Device: d}
	}
	return r
}

// SetOnChange sets the function that is called when the list changes.
func (r *Registry) SetOnChange(f func()) { r.onChange = f }

// List returns the devices, online ones first.
func (r *Registry) List() []Device {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Device, 0, len(r.devs))
	for _, e := range r.devs {
		out = append(out, e.Device)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Online != out[j].Online {
			return out[i].Online
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Get returns one device by GUID.
func (r *Registry) Get(guid string) (Device, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.devs[guid]
	if !ok {
		return Device{}, false
	}
	return e.Device, true
}

// Remove forgets a device (it comes back if it is found again).
func (r *Registry) Remove(guid string) {
	r.mu.Lock()
	delete(r.devs, guid)
	r.mu.Unlock()
	r.changed()
}

func (r *Registry) changed() {
	r.save()
	if r.onChange != nil {
		r.onChange()
	}
}

func (r *Registry) save() {
	if r.path == "" {
		return
	}
	data, err := json.MarshalIndent(r.List(), "", "  ")
	if err != nil || os.MkdirAll(filepath.Dir(r.path), 0o755) != nil {
		return
	}
	os.WriteFile(r.path, data, 0o644)
}

// Search makes the background loop search right now.
func (r *Registry) Search() {
	select {
	case r.kick <- struct{}{}:
	default:
	}
}

// Run searches until ctx ends: a 4 s listen window, then a pause, repeated. A search can be triggered with Search.
func (r *Registry) Run(ctx context.Context) {
	// The saved scanners are asked directly first, so they are online at once instead of after the first search.
	r.mu.Lock()
	var known []string
	for guid := range r.devs {
		known = append(known, guid)
	}
	r.mu.Unlock()
	if len(known) > 0 {
		r.probe(ctx, known)
		r.changed()
	}
	for {
		if r.OnSweep != nil {
			r.OnSweep(true)
		}
		r.sweep(ctx, 4*time.Second)
		if r.OnSweep != nil {
			r.OnSweep(false)
		}
		select {
		case <-ctx.Done():
			return
		case <-r.kick:
		case <-time.After(12 * time.Second):
		}
	}
}

// sweep browses for _uscan._tcp and _uscans._tcp, then probes the known devices that did not answer.
func (r *Registry) sweep(ctx context.Context, window time.Duration) {
	found := map[string]Device{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, service := range []string{"_uscan._tcp", "_uscans._tcp"} {
		wg.Add(1)
		go func(service string) {
			defer wg.Done()
			resolver, err := zeroconf.NewResolver(nil)
			if err != nil {
				return
			}
			entries := make(chan *zeroconf.ServiceEntry)
			bctx, cancel := context.WithTimeout(ctx, window)
			defer cancel()
			go func() {
				for e := range entries {
					if d, ok := deviceFromEntry(e); ok {
						mu.Lock()
						// a scanner that offers both is kept once, and plain HTTP wins: some HTTPS stacks use TLS
						// versions that are no longer supported
						if old, dup := found[d.GUID]; !dup || (old.Scheme == "https" && d.Scheme == "http") {
							found[d.GUID] = d
						}
						mu.Unlock()
					}
				}
			}()
			if resolver.Browse(bctx, service, "local.", entries) == nil {
				<-bctx.Done()
			}
		}(service)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return
	}
	mu.Lock() // a collector may still be draining the last answers
	snapshot := make(map[string]Device, len(found))
	for k, v := range found {
		snapshot[k] = v
	}
	mu.Unlock()
	found = snapshot

	now := time.Now()
	r.mu.Lock()
	for guid, d := range found {
		old := r.devs[guid]
		if old != nil && old.Manual {
			d.Manual = true
		}
		d.LastSeen = now
		r.devs[guid] = &entry{Device: d}
	}
	var unseen []string
	for guid := range r.devs {
		if _, ok := found[guid]; !ok {
			unseen = append(unseen, guid)
		}
	}
	r.mu.Unlock()

	r.probe(ctx, unseen)
	r.changed()
}

// probe asks known scanners directly (a status request) and updates whether they are online. A scanner is only
// called offline after two missed rounds.
func (r *Registry) probe(ctx context.Context, guids []string) {
	var wg sync.WaitGroup
	for _, guid := range guids {
		wg.Add(1)
		go func(guid string) {
			defer wg.Done()
			d, ok := r.Get(guid)
			if !ok {
				return
			}
			alive := false
			if c, err := NewClient(d.BaseURL()); err == nil {
				pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				_, err := c.Status(pctx)
				cancel()
				alive = err == nil
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			e, ok := r.devs[guid]
			if !ok {
				return
			}
			if alive {
				e.misses, e.Online, e.LastSeen = 0, true, time.Now()
			} else if e.misses++; e.misses >= 2 {
				e.Online = false
			}
		}(guid)
	}
	wg.Wait()
}

// Add probes an address typed by the user ("192.168.1.20", "host:8080" or a full eSCL URL) and remembers the scanner.
func (r *Registry) Add(ctx context.Context, addr string) (Device, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return Device{}, fmt.Errorf("type the address of a scanner")
	}
	var candidates []Device
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil || u.Host == "" {
			return Device{}, fmt.Errorf("bad address %q", addr)
		}
		port := u.Port()
		if port == "" {
			port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
		}
		p, _ := strconv.Atoi(port)
		candidates = append(candidates, Device{Host: u.Hostname(), Port: p, Scheme: u.Scheme, Root: strings.Trim(u.Path, "/")})
	} else {
		host, port := addr, ""
		if h, p, err := net.SplitHostPort(addr); err == nil {
			host, port = h, p
		}
		host = strings.Trim(host, "[]")
		for _, s := range []string{"http", "https"} {
			p := port
			if p == "" {
				p = map[string]string{"http": "80", "https": "443"}[s]
			}
			pn, _ := strconv.Atoi(p)
			for _, root := range []string{"eSCL", ""} {
				candidates = append(candidates, Device{Host: host, Port: pn, Scheme: s, Root: root})
			}
		}
	}
	var lastErr error
	for _, d := range candidates {
		c, err := NewClient(d.BaseURL())
		if err != nil {
			return Device{}, err
		}
		pctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		caps, err := c.Caps(pctx)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		d.Model = caps.Model
		d.Name = caps.Model
		if d.Name == "" {
			d.Name = d.Host
		}
		d.GUID = GUID(caps.UUID, d.Host, caps.Model)
		d.Online, d.Manual, d.LastSeen = true, true, time.Now()
		r.mu.Lock()
		r.devs[d.GUID] = &entry{Device: d}
		r.mu.Unlock()
		r.changed()
		return d, nil
	}
	return Device{}, fmt.Errorf("no eSCL scanner answered at %s (%v)", addr, lastErr)
}
