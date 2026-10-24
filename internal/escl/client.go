package escl

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// IsLAN reports whether host is a local-network address. Scanners are only ever contacted on the LAN: mDNS answers
// can be forged by anyone on the network, so a device must not be able to point the app at an arbitrary server.
func IsLAN(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSuffix(host, ".")), ".local") {
		return true
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
			return false
		}
	}
	return true
}

// HTTPS scanners use self-signed certificates, so those are not verified; see IsLAN for why that is acceptable.
var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 3 * time.Minute, // a scan takes its time before the first byte
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Client talks to one scanner.
type Client struct {
	Base string // e.g. http://192.168.1.20:80/eSCL, without a trailing slash
}

// NewClient refuses anything that is not on the local network.
func NewClient(base string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("bad scanner address %q", base)
	}
	if !IsLAN(u.Host) {
		return nil, fmt.Errorf("%s is not on the local network", u.Hostname())
	}
	return &Client{Base: strings.TrimRight(base, "/")}, nil
}

func (c *Client) do(ctx context.Context, method, target, contentType string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return httpClient.Do(req)
}

func (c *Client) get(ctx context.Context, path string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := c.do(ctx, http.MethodGet, c.Base+path, "", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 4<<20))
}

// Caps reads ScannerCapabilities.
func (c *Client) Caps(ctx context.Context) (*Caps, error) {
	data, err := c.get(ctx, "/ScannerCapabilities", 6*time.Second)
	if err != nil {
		return nil, err
	}
	return ParseCaps(data)
}

// Status reads ScannerStatus.
func (c *Client) Status(ctx context.Context) (Status, error) {
	data, err := c.get(ctx, "/ScannerStatus", 4*time.Second)
	if err != nil {
		return Status{}, err
	}
	return ParseStatus(data)
}

// Unreachable reports whether err is a network failure (no route, refused, timed out) rather than an answer from the scanner.
func Unreachable(err error) bool {
	var ne net.Error
	return errors.As(err, &ne)
}

// ErrNoPages means the job ended without delivering a page.
var ErrNoPages = errors.New("the scanner returned no pages (is there a document?)")

func (c *Client) checkReady(ctx context.Context, source string) error {
	st, err := c.Status(ctx)
	if err != nil {
		return errors.New("the scanner is not reachable")
	}
	if source == "feeder" {
		switch st.AdfState {
		case "", "ScannerAdfLoaded":
		case "ScannerAdfEmpty":
			return errors.New("the feeder is empty: load paper into the document feeder and try again")
		default:
			return fmt.Errorf("feeder problem: %s", st.AdfState)
		}
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Scan runs one job and hands every page (a JPEG) to onPage as soon as the scanner delivers it. Cancelling ctx
// cancels the job on the scanner. It returns the number of pages delivered.
func (c *Client) Scan(ctx context.Context, r Resolved, onPage func(jpeg []byte)) (int, error) {
	if err := c.checkReady(ctx, r.Source); err != nil {
		return 0, err
	}
	body := []byte(SettingsXML(r))

	// The scanner answers 503 while it is busy (e.g. finishing the previous job), so retry briefly.
	var res *http.Response
	for attempt := 1; ; attempt++ {
		var err error
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		// Brother scanners ignore the whole request body unless this is application/xml, not text/xml.
		res, err = c.do(cctx, http.MethodPost, c.Base+"/ScanJobs", "application/xml", body)
		cancel()
		if err != nil {
			if Unreachable(err) {
				return 0, errors.New("could not reach the scanner: is it switched on and on the same network?")
			}
			return 0, fmt.Errorf("could not start the scan: %w", err)
		}
		if res.StatusCode != http.StatusServiceUnavailable || attempt == 6 {
			break
		}
		res.Body.Close()
		if err := sleep(ctx, 2*time.Second); err != nil {
			return 0, err
		}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if res.StatusCode == http.StatusServiceUnavailable {
			return 0, errors.New("the scanner is busy: wait a moment and try again")
		}
		return 0, fmt.Errorf("the scanner rejected the job (HTTP %d %s)", res.StatusCode, strings.Join(strings.Fields(string(msg)), " "))
	}
	loc := res.Header.Get("Location")
	if loc == "" {
		return 0, errors.New("the scanner returned no job location")
	}
	base, _ := url.Parse(c.Base)
	ju, err := base.Parse(loc)
	if err != nil || ju.Host != base.Host {
		return 0, errors.New("the scanner returned a job location on another host")
	}
	job := strings.TrimRight(ju.String(), "/")
	defer func() {
		// the job is deleted even when the scan was cancelled
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if dr, err := c.do(dctx, http.MethodDelete, job, "", nil); err == nil {
			dr.Body.Close()
		}
	}()

	pages := 0
	misses := 0
	for {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		pr, err := c.do(pctx, http.MethodGet, job+"/NextDocument", "", nil)
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return pages, ctx.Err()
			}
			return pages, fmt.Errorf("lost the scanner while reading a page: %w", err)
		}
		switch pr.StatusCode {
		case http.StatusOK:
			data, err := io.ReadAll(pr.Body)
			pr.Body.Close()
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return pages, ctx.Err()
				}
				return pages, fmt.Errorf("lost the scanner while reading a page: %w", err)
			}
			pages++
			misses = 0
			onPage(data)
			if r.Source == "platen" {
				return pages, nil // the flatbed yields one page per job
			}
		case http.StatusNotFound, http.StatusConflict, http.StatusServiceUnavailable:
			pr.Body.Close()
			cancel()
			// 404 means "no more pages", but some scanners (Brother) also answer it for a request that comes right
			// after the previous page while the next sheet is still feeding. So: try once more shortly, then ask
			// the scanner whether it is still processing.
			misses++
			if misses >= 2 {
				if st, err := c.Status(ctx); err != nil || st.State != "Processing" || misses > 60 {
					if pages == 0 {
						return 0, ErrNoPages
					}
					return pages, nil
				}
			}
			if err := sleep(ctx, 500*time.Millisecond); err != nil {
				return pages, err
			}
		default:
			pr.Body.Close()
			cancel()
			return pages, fmt.Errorf("the scanner failed while reading a page (HTTP %d)", pr.StatusCode)
		}
	}
}
