package management

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"

	"golang.org/x/net/idna"
)

type policySpec struct {
	Name       string             `json:"name"`
	EngineMode string             `json:"engineMode"`
	CRS        crsSettings        `json:"crs"`
	Inspection inspectionSettings `json:"inspection"`
	RateLimit  rateSettings       `json:"rateLimit"`
}

type crsSettings struct {
	ParanoiaLevel            int `json:"paranoiaLevel"`
	InboundAnomalyThreshold  int `json:"inboundAnomalyThreshold"`
	OutboundAnomalyThreshold int `json:"outboundAnomalyThreshold"`
}

type inspectionSettings struct {
	RequestBodyEnabled           bool   `json:"requestBodyEnabled"`
	RequestBodyLimitBytes        int64  `json:"requestBodyLimitBytes"`
	RequestBodyNoFilesLimitBytes int64  `json:"requestBodyNoFilesLimitBytes"`
	OversizeAction               string `json:"oversizeAction"`
	ParserErrorAction            string `json:"parserErrorAction"`
	ResponseBodyEnabled          bool   `json:"responseBodyEnabled"`
}

type rateSettings struct {
	Enabled           bool `json:"enabled"`
	RequestsPerSecond int  `json:"requestsPerSecond"`
	Burst             int  `json:"burst"`
}

func defaultPolicy() policySpec {
	return policySpec{EngineMode: "detectionOnly", CRS: crsSettings{1, 5, 4},
		Inspection: inspectionSettings{true, 10 << 20, 1 << 20, "reject", "reject", false},
		RateLimit:  rateSettings{false, 10, 20}}
}

func validatePolicy(p policySpec) string {
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 256 || (p.EngineMode != "off" && p.EngineMode != "detectionOnly" && p.EngineMode != "blocking") {
		return "VALIDATION_FAILED"
	}
	if p.CRS.ParanoiaLevel < 1 || p.CRS.ParanoiaLevel > 4 || p.CRS.InboundAnomalyThreshold < 1 || p.CRS.InboundAnomalyThreshold > 100 || p.CRS.OutboundAnomalyThreshold < 1 || p.CRS.OutboundAnomalyThreshold > 100 {
		return "VALIDATION_FAILED"
	}
	i := p.Inspection
	if i.RequestBodyLimitBytes < 1 || i.RequestBodyLimitBytes > 100<<20 || i.RequestBodyNoFilesLimitBytes < 1 || i.RequestBodyNoFilesLimitBytes > i.RequestBodyLimitBytes {
		return "VALIDATION_FAILED"
	}
	if (i.OversizeAction != "reject" && i.OversizeAction != "processPartial") || (i.ParserErrorAction != "reject" && i.ParserErrorAction != "logOnly") {
		return "VALIDATION_FAILED"
	}
	// Response inspection is outside the supported initial configuration surface.
	if i.ResponseBodyEnabled {
		return "UNSUPPORTED_INSPECTION_SETTING"
	}
	if p.RateLimit.RequestsPerSecond < 1 || p.RateLimit.RequestsPerSecond > 10000 || p.RateLimit.Burst < 0 || p.RateLimit.Burst > 100000 {
		return "VALIDATION_FAILED"
	}
	return ""
}

type websiteSpec struct {
	Name          string        `json:"name"`
	Domains       []string      `json:"domains"`
	Enabled       bool          `json:"enabled"`
	Listeners     siteListeners `json:"listeners"`
	CertificateID *string       `json:"certificateId"`
	PolicyID      string        `json:"policyId"`
	Upstreams     []upstream    `json:"upstreams"`
	UpstreamTLS   upstreamTLS   `json:"upstreamTls"`
	Timeouts      siteTimeouts  `json:"timeouts"`
}

type siteListeners struct {
	HTTP                bool `json:"http"`
	HTTPS               bool `json:"https"`
	RedirectHTTPToHTTPS bool `json:"redirectHttpToHttps"`
}

type upstream struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

type upstreamTLS struct {
	Verify        bool    `json:"verify"`
	ServerName    string  `json:"serverName"`
	TrustSource   string  `json:"trustSource"`
	TrustBundleID *string `json:"trustBundleId"`
}

type siteTimeouts struct {
	ConnectSeconds int `json:"connectSeconds"`
	ReadSeconds    int `json:"readSeconds"`
}

func defaultWebsite() websiteSpec {
	return websiteSpec{Enabled: true, Listeners: siteListeners{HTTP: true}, UpstreamTLS: upstreamTLS{Verify: true, TrustSource: "system"}, Timeouts: siteTimeouts{5, 60}}
}

// normalizeHostname accepts IDNA names and returns a single canonical spelling.
// Address literals, URI syntax, wildcards and directives are never DNS names.
func normalizeHostname(host string, site bool) (string, error) {
	if host == "" || strings.ContainsAny(host, "/\\:@*% \t\r\n\x00") {
		return "", errors.New("invalid hostname")
	}
	ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(host, "."))
	if err != nil {
		return "", err
	}
	ascii = strings.ToLower(ascii)
	if len(ascii) > 253 || ascii == "localhost" || strings.HasSuffix(ascii, ".localhost") || site && !strings.Contains(ascii, ".") {
		return "", errors.New("invalid hostname")
	}
	if _, err := netip.ParseAddr(ascii); err == nil {
		return "", errors.New("address literal is not a hostname")
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid hostname label")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", errors.New("invalid hostname label")
			}
		}
	}
	return ascii, nil
}

func normalizeWebsite(v *websiteSpec) string {
	if strings.TrimSpace(v.Name) == "" || len(v.Name) > 256 || len(v.Domains) < 1 || len(v.Domains) > 20 || len(v.PolicyID) != 43 {
		return "VALIDATION_FAILED"
	}
	if !v.Listeners.HTTP && !v.Listeners.HTTPS || v.Listeners.RedirectHTTPToHTTPS && (!v.Listeners.HTTP || !v.Listeners.HTTPS) {
		return "VALIDATION_FAILED"
	}
	// Never accept a pretend certificate reference before certificate storage exists.
	if v.Listeners.HTTPS || v.CertificateID != nil {
		return "CERTIFICATES_UNAVAILABLE"
	}
	seen := map[string]bool{}
	for i, domain := range v.Domains {
		normalized, err := normalizeHostname(domain, true)
		if err != nil || seen[normalized] {
			return "VALIDATION_FAILED"
		}
		seen[normalized] = true
		v.Domains[i] = normalized
	}
	if len(v.Upstreams) < 1 || len(v.Upstreams) > 10 {
		return "VALIDATION_FAILED"
	}
	for i, u := range v.Upstreams {
		if u.Scheme != "http" && u.Scheme != "https" || u.Port < 1 || u.Port > 65535 {
			return "VALIDATION_FAILED"
		}
		if addr, err := netip.ParseAddr(u.Host); err == nil {
			if addr.Zone() != "" {
				return "VALIDATION_FAILED"
			}
			v.Upstreams[i].Host = addr.Unmap().String()
		} else {
			host, err := normalizeHostname(u.Host, false)
			if err != nil {
				return "VALIDATION_FAILED"
			}
			v.Upstreams[i].Host = host
		}
	}
	if !v.UpstreamTLS.Verify {
		return "UPSTREAM_TLS_VERIFICATION_REQUIRED"
	}
	if v.UpstreamTLS.TrustSource != "system" || v.UpstreamTLS.TrustBundleID != nil {
		return "TRUST_BUNDLES_UNAVAILABLE"
	}
	if v.UpstreamTLS.ServerName != "" {
		name, err := normalizeHostname(v.UpstreamTLS.ServerName, false)
		if err != nil {
			return "VALIDATION_FAILED"
		}
		v.UpstreamTLS.ServerName = name
	}
	if v.Timeouts.ConnectSeconds < 1 || v.Timeouts.ConnectSeconds > 30 || v.Timeouts.ReadSeconds < 1 || v.Timeouts.ReadSeconds > 300 {
		return "VALIDATION_FAILED"
	}
	return ""
}

type proxySpec struct {
	AllowedCIDRs      []string `json:"allowedCidrs"`
	TrustedProxyCIDRs []string `json:"trustedProxyCidrs"`
}

func normalizeProxy(p *proxySpec) string {
	for _, items := range []*[]string{&p.AllowedCIDRs, &p.TrustedProxyCIDRs} {
		if *items == nil || len(*items) > 64 {
			return "VALIDATION_FAILED"
		}
		seen := map[string]bool{}
		for i, raw := range *items {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil || prefix.Addr().Is4In6() {
				return "VALIDATION_FAILED"
			}
			canonical := prefix.Masked().String()
			if seen[canonical] {
				return "VALIDATION_FAILED"
			}
			seen[canonical] = true
			(*items)[i] = canonical
		}
	}
	return ""
}

var errUnknownField = errors.New("unknown field")
var errInvalidField = errors.New("invalid field")

// decodeSpec merges nested PATCH objects with the stored typed definition.
// The shape is derived only from writable fields, so metadata cannot be assigned.
func decodeSpec[T any](w http.ResponseWriter, r *http.Request, base T, patch bool) (T, bool) {
	var raw map[string]json.RawMessage
	if !decode(w, r, &raw) {
		return base, false
	}
	if patch && len(raw) == 0 {
		failure(w, r, 422, "VALIDATION_FAILED")
		return base, false
	}
	encoded, _ := json.Marshal(base)
	var current map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &current); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return base, false
	}
	if err := mergeSpec(current, raw, 0); err != nil {
		if errors.Is(err, errUnknownField) {
			failure(w, r, 400, "UNKNOWN_FIELD")
		} else {
			failure(w, r, 422, "VALIDATION_FAILED")
		}
		return base, false
	}
	encoded, _ = json.Marshal(current)
	d := json.NewDecoder(bytes.NewReader(encoded))
	d.DisallowUnknownFields()
	var result T
	if err := d.Decode(&result); err != nil {
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			failure(w, r, 400, "UNKNOWN_FIELD")
		} else {
			failure(w, r, 422, "VALIDATION_FAILED")
		}
		return base, false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		failure(w, r, 400, "INVALID_JSON")
		return base, false
	}
	return result, true
}

func mergeSpec(current, patch map[string]json.RawMessage, depth int) error {
	for key, value := range patch {
		old, exists := current[key]
		if !exists {
			return errUnknownField
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if depth == 0 && key == "certificateId" || depth == 1 && key == "trustBundleId" {
				current[key] = value
				continue
			}
			return errInvalidField
		}
		var nested map[string]json.RawMessage
		if len(old) > 0 && old[0] == '{' {
			if err := json.Unmarshal(old, &nested); err != nil {
				return errInvalidField
			}
			var changes map[string]json.RawMessage
			if err := json.Unmarshal(value, &changes); err != nil || changes == nil {
				return errInvalidField
			}
			if err := mergeSpec(nested, changes, depth+1); err != nil {
				return err
			}
			merged, _ := json.Marshal(nested)
			current[key] = merged
		} else {
			current[key] = value
		}
	}
	return nil
}
