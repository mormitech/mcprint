package escl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Item is one line of the additional info. URL, if set, makes the value a link.
type Item struct {
	Label string `json:"label"`
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

// Section is a titled group of items.
type Section struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

// Extra is everything else a scanner will tell. Every source is optional: a scanner may offer all, some or none of it,
// and Sections is simply empty when it offers nothing.
type Extra struct {
	Icon     string    `json:"icon,omitempty"` // a data: URL of the picture the scanner offers of itself
	Sections []Section `json:"sections"`
}

// vendorPages are well-known pages where printers keep usage counters and the like. Each is one plain GET, and a printer
// that does not have the page answers 404 and the section is left out.
var vendorPages = []struct{ title, path, kind string }{
	{"Counters and consumables", "/etc/mnt_info.csv", "csv"},    // Brother
	{"Usage", "/DevMgmt/ProductUsageDyn.xml", "xml"},            // HP
	{"Consumables", "/DevMgmt/ConsumableConfigDyn.xml", "xml"},  // HP
	{"Product details", "/DevMgmt/ProductConfigDyn.xml", "xml"}, // HP
}

const (
	maxItems = 120
	maxBody  = 2 << 20
)

// node is any XML element, so documents we know nothing about can still be shown.
type node struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
	Kids    []node `xml:",any"`
}

func parseNode(data []byte) (node, bool) {
	var n node
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	if d.Decode(&n) != nil {
		return node{}, false
	}
	return n, true
}

func (n node) kid(name string) *node {
	for i := range n.Kids {
		if n.Kids[i].XMLName.Local == name {
			return &n.Kids[i]
		}
	}
	return nil
}

// humanize turns "ImagesCompleted" into "Images completed".
func humanize(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(rune(s[i-1])) {
			b.WriteByte(' ')
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// leaves lists the elements below n that hold text, labelled by their path. Names in skip (at any depth) are left out.
func (n node) leaves(prefix []string, skip map[string]bool, out *[]Item) {
	for _, k := range n.Kids {
		name := k.XMLName.Local
		if skip[name] || len(*out) >= maxItems {
			continue
		}
		path := append(append([]string{}, prefix...), humanize(name))
		if len(k.Kids) > 0 {
			k.leaves(path, skip, out)
			continue
		}
		if v := strings.TrimSpace(k.Text); v != "" {
			*out = append(*out, Item{Label: strings.Join(path, " › "), Value: v})
		}
	}
}

// duration says "3 h 20 min" for a number of seconds.
func duration(sec int) string {
	switch d := time.Duration(sec) * time.Second; {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d s", sec)
	}
}

// fetch is a plain GET of a path on the scanner's own host (never another one, and redirects are not followed).
func (c *Client) fetch(ctx context.Context, target string, timeout time.Duration) ([]byte, string, error) {
	base, err := url.Parse(c.Base)
	if err != nil {
		return nil, "", err
	}
	u, err := base.Parse(target)
	if err != nil || u.Hostname() != base.Hostname() {
		return nil, "", fmt.Errorf("not on the scanner's own address: %s", target)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := c.do(ctx, http.MethodGet, u.String(), "", nil)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	return data, res.Header.Get("Content-Type"), err
}

// root is the scanner's web server, not its eSCL path.
func (c *Client) root(path string) string {
	u, _ := url.Parse(c.Base)
	u.Path, u.RawQuery = path, ""
	return u.String()
}

// text turns bytes of an unknown encoding into UTF-8: printers often send a legacy 8-bit code page.
func text(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// parseCSV reads a header row and a row of values (a counters file) as label and value pairs, leaving out empty values.
func parseCSV(data []byte) []Item {
	r := csv.NewReader(strings.NewReader(text(data)))
	r.FieldsPerRecord, r.LazyQuotes = -1, true
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil
	}
	var out []Item
	for i, h := range rows[0] {
		if i >= len(rows[1]) || strings.TrimSpace(h) == "" || strings.TrimSpace(rows[1][i]) == "" || len(out) >= maxItems {
			continue
		}
		out = append(out, Item{Label: strings.TrimSpace(h), Value: strings.TrimSpace(rows[1][i])})
	}
	return out
}

// safeLink keeps only web addresses on the local network, so a scanner cannot send the user somewhere else.
func safeLink(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || !IsLAN(u.Host) {
		return ""
	}
	return u.String()
}

// Gather asks the scanner for everything it offers beyond what the app uses. Failures are not errors: they only mean
// that the scanner does not have that information.
func Gather(ctx context.Context, d Device) Extra {
	extra := Extra{Sections: []Section{}}
	c, err := NewClient(d.BaseURL())
	if err != nil {
		return extra
	}

	var (
		wg                   sync.WaitGroup
		caps, status         []byte
		vendor               = make([][]byte, len(vendorPages))
		web                  string
		capsSkip             = map[string]bool{"MakeAndModel": true, "Manufacturer": true, "UUID": true, "Version": true, "Platen": true, "Adf": true, "Camera": true, "IconURI": true, "AdminURI": true}
		capsRoot, statusRoot node
		haveCaps, haveStatus bool
		adminURI, iconURI    string
		links                []Item
	)
	run := func(f func()) { wg.Add(1); go func() { defer wg.Done(); f() }() }
	run(func() { caps, _, _ = c.fetch(ctx, c.Base+"/ScannerCapabilities", 6*time.Second) })
	run(func() { status, _, _ = c.fetch(ctx, c.Base+"/ScannerStatus", 5*time.Second) })
	for i, v := range vendorPages {
		run(func() { vendor[i], _, _ = c.fetch(ctx, c.root(v.path), 5*time.Second) })
	}
	run(func() { // does it have a web interface at all?
		if _, _, err := c.fetch(ctx, c.root("/"), 4*time.Second); err == nil || strings.HasPrefix(err.Error(), "HTTP 3") {
			web = c.root("/")
		}
	})
	wg.Wait()

	// What the scanner announced with mDNS.
	if len(d.Info) > 0 {
		keys := make([]string, 0, len(d.Info))
		for k, v := range d.Info {
			if strings.TrimSpace(v) != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		var items []Item
		for _, k := range keys {
			items = append(items, Item{Label: k, Value: d.Info[k]})
		}
		if len(items) > 0 {
			extra.Sections = append(extra.Sections, Section{"Announced on the network (mDNS)", items})
		}
		adminURI = d.Info["adminurl"]
		iconURI = d.Info["representation"]
	}

	if capsRoot, haveCaps = parseNode(caps); haveCaps {
		if n := capsRoot.kid("AdminURI"); n != nil && adminURI == "" {
			adminURI = strings.TrimSpace(n.Text)
		}
		if n := capsRoot.kid("IconURI"); n != nil && strings.TrimSpace(n.Text) != "" {
			iconURI = strings.TrimSpace(n.Text)
		}
		var items []Item
		capsRoot.leaves(nil, capsSkip, &items)
		if len(items) > 0 {
			extra.Sections = append(extra.Sections, Section{"More from the capabilities document", items})
		}
	}

	if statusRoot, haveStatus = parseNode(status); haveStatus {
		var items []Item
		statusRoot.leaves(nil, map[string]bool{"Jobs": true, "State": true, "AdfState": true, "Version": true}, &items)
		if len(items) > 0 {
			extra.Sections = append(extra.Sections, Section{"More from the status", items})
		}
		if jobs := statusRoot.kid("Jobs"); jobs != nil && len(jobs.Kids) > 0 {
			var list []Item
			for i, j := range jobs.Kids {
				var f []Item
				j.leaves(nil, map[string]bool{"JobUri": true}, &f)
				lines := make([]string, 0, len(f))
				for _, it := range f {
					v, label := it.Value, it.Label
					if i := strings.LastIndex(label, " › "); i >= 0 {
						label = label[i+len(" › "):]
					}
					switch label {
					case "Age":
						if s, err := strconv.Atoi(v); err == nil {
							v = duration(s) + " ago"
						}
					case "Job uuid":
						v = strings.TrimPrefix(v, "urn:uuid:")
					case "Job state reason", "Job state":
						v = humanize(strings.TrimPrefix(v, "Job"))
					}
					lines = append(lines, strings.ToLower(label[:1])+label[1:]+": "+v)
				}
				list = append(list, Item{Label: "Job " + strconv.Itoa(i+1), Value: strings.Join(lines, "\n")})
			}
			extra.Sections = append(extra.Sections, Section{"Jobs the scanner remembers", list})
		}
	}

	for i, v := range vendorPages {
		var items []Item
		switch v.kind {
		case "csv":
			items = parseCSV(vendor[i])
		case "xml":
			if n, ok := parseNode(vendor[i]); ok {
				n.leaves(nil, nil, &items)
			}
		}
		if len(items) > 0 {
			extra.Sections = append(extra.Sections, Section{v.title, items})
		}
	}

	// Links, then the picture.
	if l := safeLink(adminURI); l != "" {
		links = append(links, Item{Label: "Administration page", Value: l, URL: l})
	}
	if web != "" && (len(links) == 0 || links[0].URL != web) {
		links = append(links, Item{Label: "Web interface", Value: web, URL: web})
	}
	if len(links) > 0 {
		extra.Sections = append([]Section{{"Links", links}}, extra.Sections...)
	}
	extra.Icon = c.iconFrom(ctx, iconURI)
	return extra
}

// iconFrom fetches the picture a scanner offers of itself (its "representation" or IconURI) as a data: URL, or "" if
// there is none. Scanners often name themselves in that address ("http://BRN1234.local./icons/x.png"), which not every
// system can resolve, so the picture is fetched from the address the scanner was found at, with the path it gave.
func (c *Client) iconFrom(ctx context.Context, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	base, err := url.Parse(c.Base)
	if err != nil {
		return ""
	}
	u, err := base.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	if u.Hostname() != base.Hostname() {
		if !IsLAN(u.Host) {
			return ""
		}
		u.Host = base.Hostname()
		if strings.Contains(u.Host, ":") {
			u.Host = "[" + u.Host + "]"
		}
		if base.Scheme == u.Scheme && base.Port() != "" {
			u.Host += ":" + base.Port()
		}
	}
	data, ctype, err := c.fetch(ctx, u.String(), 5*time.Second)
	if err != nil || len(data) == 0 || !strings.HasPrefix(ctype, "image/") {
		return ""
	}
	return "data:" + ctype + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// Icon is the picture a scanner offers of itself, from its mDNS announcement or else its capabilities, or "".
func Icon(ctx context.Context, d Device) string {
	c, err := NewClient(d.BaseURL())
	if err != nil {
		return ""
	}
	if raw := d.Info["representation"]; raw != "" {
		if icon := c.iconFrom(ctx, raw); icon != "" {
			return icon
		}
	}
	data, _, err := c.fetch(ctx, c.Base+"/ScannerCapabilities", 6*time.Second)
	if err != nil {
		return ""
	}
	if n, ok := parseNode(data); ok {
		if k := n.kid("IconURI"); k != nil {
			return c.iconFrom(ctx, k.Text)
		}
	}
	return ""
}
