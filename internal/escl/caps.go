// Package escl is a client for eSCL (AirScan): finding scanners with mDNS, reading what they can do and running scan jobs.
package escl

import (
	"encoding/xml"
	"errors"
	"strings"
)

// Resolutions is what a scanner offers: a list of values, or a range with a step.
type Resolutions struct {
	Values []int `json:"values"`
	Min    int   `json:"min"`
	Max    int   `json:"max"`
	Step   int   `json:"step"`
}

// Input is one way of feeding paper: flatbed, feeder simplex or feeder duplex. Sizes are in 1/300 inch.
type Input struct {
	ColorModes  []string    `json:"colorModes"`
	Formats     []string    `json:"formats"`
	Resolutions Resolutions `json:"resolutions"`
	MinWidth    int         `json:"minWidth"`
	MaxWidth    int         `json:"maxWidth"`
	MinHeight   int         `json:"minHeight"`
	MaxHeight   int         `json:"maxHeight"`
}

// Caps is the parsed ScannerCapabilities document.
type Caps struct {
	Make           string `json:"make"`
	Model          string `json:"model"`
	UUID           string `json:"uuid"`
	Version        string `json:"version"`
	Platen         *Input `json:"platen"`
	Simplex        *Input `json:"simplex"`
	Duplex         *Input `json:"duplex"`
	FeederCapacity int    `json:"feederCapacity"`
}

// Status is the ScannerStatus document.
type Status struct {
	State    string `json:"state"`
	AdfState string `json:"adfState"`
	Jobs     []Job  `json:"jobs"`
}

// Job is one entry of the scanner's job list (running, finished or cancelled; scanners keep the last ones).
type Job struct {
	URI     string   `json:"uri"`
	State   string   `json:"state"`   // Pending, Processing, Completed, Canceled, Aborted
	Reasons []string `json:"reasons"` // for example JobCanceledAtDevice
	Done    int      `json:"done"`    // images completed
	ToSend  int      `json:"toSend"`  // images still to transfer
}

type xmlInput struct {
	MinWidth  int `xml:"MinWidth"`
	MaxWidth  int `xml:"MaxWidth"`
	MinHeight int `xml:"MinHeight"`
	MaxHeight int `xml:"MaxHeight"`
	Profiles  []struct {
		ColorModes []string `xml:"ColorModes>ColorMode"`
		Formats    []string `xml:"DocumentFormats>DocumentFormat"`
		FormatsExt []string `xml:"DocumentFormats>DocumentFormatExt"`
		Discrete   []struct {
			X int `xml:"XResolution"`
			Y int `xml:"YResolution"`
		} `xml:"SupportedResolutions>DiscreteResolutions>DiscreteResolution"`
		Range *struct {
			Min  int `xml:"Min"`
			Max  int `xml:"Max"`
			Step int `xml:"Step"`
		} `xml:"SupportedResolutions>ResolutionRange>XResolutionRange"`
	} `xml:"SettingProfiles>SettingProfile"`
}

type xmlCaps struct {
	Version      string `xml:"Version"`
	MakeAndModel string `xml:"MakeAndModel"`
	Manufacturer string `xml:"Manufacturer"`
	UUID         string `xml:"UUID"`
	Platen       *struct {
		Input *xmlInput `xml:"PlatenInputCaps"`
	} `xml:"Platen"`
	Adf *struct {
		Simplex  *xmlInput `xml:"AdfSimplexInputCaps"`
		Duplex   *xmlInput `xml:"AdfDuplexInputCaps"`
		Capacity int       `xml:"FeederCapacity"`
	} `xml:"Adf"`
}

func uniqAppend(list []string, more ...string) []string {
	for _, m := range more {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		found := false
		for _, l := range list {
			if l == m {
				found = true
				break
			}
		}
		if !found {
			list = append(list, m)
		}
	}
	return list
}

func (x *xmlInput) input() *Input {
	if x == nil {
		return nil
	}
	in := &Input{
		ColorModes: []string{}, Formats: []string{},
		MinWidth: x.MinWidth, MaxWidth: x.MaxWidth, MinHeight: x.MinHeight, MaxHeight: x.MaxHeight,
	}
	in.Resolutions.Values = []int{}
	seen := map[int]bool{}
	for _, p := range x.Profiles {
		in.ColorModes = uniqAppend(in.ColorModes, p.ColorModes...)
		in.Formats = uniqAppend(in.Formats, p.Formats...)
		in.Formats = uniqAppend(in.Formats, p.FormatsExt...)
		for _, d := range p.Discrete {
			if d.X == d.Y && d.X > 0 && !seen[d.X] {
				seen[d.X] = true
				in.Resolutions.Values = append(in.Resolutions.Values, d.X)
			}
		}
		if p.Range != nil && in.Resolutions.Max == 0 && p.Range.Max > 0 {
			in.Resolutions.Min, in.Resolutions.Max, in.Resolutions.Step = p.Range.Min, p.Range.Max, p.Range.Step
			if in.Resolutions.Step <= 0 {
				in.Resolutions.Step = 1
			}
		}
	}
	// ascending
	v := in.Resolutions.Values
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
	return in
}

// ParseCaps parses a ScannerCapabilities document.
func ParseCaps(data []byte) (*Caps, error) {
	var x xmlCaps
	if err := xml.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	if x.Platen == nil && x.Adf == nil {
		return nil, errors.New("not an eSCL capabilities document")
	}
	c := &Caps{
		Make:    strings.TrimSpace(x.Manufacturer),
		Model:   strings.TrimSpace(x.MakeAndModel),
		UUID:    strings.TrimSpace(x.UUID),
		Version: strings.TrimSpace(x.Version),
	}
	if x.Platen != nil {
		c.Platen = x.Platen.Input.input()
	}
	if x.Adf != nil {
		c.Simplex = x.Adf.Simplex.input()
		c.Duplex = x.Adf.Duplex.input()
		c.FeederCapacity = x.Adf.Capacity
	}
	return c, nil
}

// ParseStatus parses a ScannerStatus document.
func ParseStatus(data []byte) (Status, error) {
	var x struct {
		State    string `xml:"State"`
		AdfState string `xml:"AdfState"`
		Jobs     []struct {
			URI     string   `xml:"JobUri"`
			State   string   `xml:"JobState"`
			Reasons []string `xml:"JobStateReasons>JobStateReason"`
			Done    int      `xml:"ImagesCompleted"`
			ToSend  int      `xml:"ImagesToTransfer"`
		} `xml:"Jobs>JobInfo"`
	}
	if err := xml.Unmarshal(data, &x); err != nil {
		return Status{}, err
	}
	st := Status{State: strings.TrimSpace(x.State), AdfState: strings.TrimSpace(x.AdfState), Jobs: []Job{}}
	for _, j := range x.Jobs {
		job := Job{URI: strings.TrimSpace(j.URI), State: strings.TrimSpace(j.State), Reasons: []string{}, Done: j.Done, ToSend: j.ToSend}
		for _, r := range j.Reasons {
			job.Reasons = append(job.Reasons, strings.TrimSpace(r))
		}
		st.Jobs = append(st.Jobs, job)
	}
	return st, nil
}
