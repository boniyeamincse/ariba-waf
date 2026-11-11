package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/netip"
)

// These ranges remain forbidden even when an Admin supplies a broad allowlist.
// IPv6 translation/tunnel prefixes are excluded to avoid hidden IPv4 targets.
var forbiddenDestinations = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("100.100.100.200/32"),
	netip.MustParsePrefix("168.63.129.16/32"), netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("fd00:ec2::254/128"),
}

func allowedAddress(addr netip.Addr, allowed []netip.Prefix) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.Zone() != "" || !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range forbiddenDestinations {
		if prefix.Contains(addr) {
			return false
		}
	}
	for _, prefix := range allowed {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// validateUpstreams checks saved desired state only. A future activation/dialer
// must repeat these checks and connect to the validated address, without a second
// unchecked DNS resolution. Saving this result never authorizes a later dial.
func (s *Server) validateUpstreams(ctx context.Context, upstreams []upstream, settings proxySpec) string {
	allowed := make([]netip.Prefix, 0, len(settings.AllowedCIDRs))
	for _, raw := range settings.AllowedCIDRs {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return "DESTINATION_FORBIDDEN"
		}
		allowed = append(allowed, prefix)
	}
	if len(allowed) == 0 {
		return "DESTINATION_FORBIDDEN"
	}
	for _, u := range upstreams {
		// Conservatively reserve management ports until the integrated runtime can
		// supply a complete inventory of its own listener addresses.
		if u.Port == 9000 || u.Port == 9443 {
			return "DESTINATION_FORBIDDEN"
		}
		var addresses []netip.Addr
		if address, err := netip.ParseAddr(u.Host); err == nil {
			addresses = []netip.Addr{address}
		} else {
			addresses, err = s.resolver.LookupNetIP(ctx, "ip", u.Host)
			if err != nil || len(addresses) == 0 || len(addresses) > 64 {
				return "DESTINATION_UNRESOLVED"
			}
		}
		for _, address := range addresses {
			if !allowedAddress(address, allowed) {
				return "DESTINATION_FORBIDDEN"
			}
		}
	}
	return ""
}

func loadProxy(ctx context.Context, tx *sql.Tx) (proxySpec, int, error) {
	var body string
	var version int
	var spec proxySpec
	if err := tx.QueryRowContext(ctx, "SELECT body,version FROM proxy_settings WHERE id=1").Scan(&body, &version); err != nil {
		return spec, 0, err
	}
	err := json.Unmarshal([]byte(body), &spec)
	return spec, version, err
}
