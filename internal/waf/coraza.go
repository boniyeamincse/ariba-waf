package waf

import (
	"errors"
	"fmt"
	"net/http"

	corazaconf "github.com/corazawaf/coraza-coreruleset/coraza/v3"
	crs "github.com/corazawaf/coraza-coreruleset/crs/v4"
	"github.com/corazawaf/coraza/v3"
	corazahttp "github.com/corazawaf/coraza/v3/http"
	"github.com/jcchavezs/mergefs"
)

// Mode selects the rule-engine behavior for this exploratory Coraza adapter.
type Mode string

const (
	ModeOff       Mode = "off"
	ModeDetection Mode = "detectionOnly"
	ModeBlocking  Mode = "blocking"

	requestBodyLimit       = 1 << 20
	requestBodyMemoryLimit = 128 << 10
)

// NewCorazaHandler creates a CRS-backed handler for local prototype and tests.
// It does not configure Nginx, persistence, audit-event storage, or production
// upstream policy; callers must supply the next handler explicitly.
func NewCorazaHandler(next http.Handler, mode Mode) (http.Handler, error) {
	if next == nil {
		return nil, errors.New("next handler is required")
	}

	engineMode, err := directiveFor(mode)
	if err != nil {
		return nil, err
	}

	directives := fmt.Sprintf(`
Include @coraza.conf-recommended
SecRuleEngine %s
Include @crs-setup.conf.example
Include @owasp_crs/*.conf
`, engineMode)

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithRootFS(mergefs.Merge(crs.FS, corazaconf.FS)).
			WithRequestBodyAccess().
			WithRequestBodyLimit(requestBodyLimit).
			WithRequestBodyInMemoryLimit(requestBodyMemoryLimit).
			WithDirectives(directives),
	)
	if err != nil {
		return nil, fmt.Errorf("load Coraza CRS configuration: %w", err)
	}

	return corazahttp.WrapHandler(waf, next), nil
}

func directiveFor(mode Mode) (string, error) {
	switch mode {
	case ModeOff:
		return "Off", nil
	case ModeDetection:
		return "DetectionOnly", nil
	case ModeBlocking:
		return "On", nil
	default:
		return "", fmt.Errorf("unsupported WAF mode %q", mode)
	}
}
