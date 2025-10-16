package config

import (
	"net"
	"os"
	"strings"
)

// AllowedNetworks returns CIDR blocks from SMTP_ALLOW_NETWORKS.
func AllowedNetworks() []*net.IPNet {
	value := strings.TrimSpace(os.Getenv("SMTP_ALLOW_NETWORKS"))
	if value == "" {
		return nil
	}
	var result []*net.IPNet
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			if ip := net.ParseIP(part); ip != nil {
				mask := net.CIDRMask(len(ip)*8, len(ip)*8)
				network := &net.IPNet{IP: ip, Mask: mask}
				result = append(result, network)
			}
			continue
		}
		if _, network, err := net.ParseCIDR(part); err == nil {
			result = append(result, network)
		}
	}
	return result
}

// AllowedHosts returns exact hostnames from SMTP_ALLOW_HOSTS.
func AllowedHosts() []string {
	value := strings.TrimSpace(os.Getenv("SMTP_ALLOW_HOSTS"))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var hosts []string
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		hosts = append(hosts, part)
	}
	return hosts
}

// RequireSenderDomain reports whether SMTP_REQUIRE_LOCAL_DOMAIN is enabled.
func RequireSenderDomain() bool {
	return Bool("SMTP_REQUIRE_LOCAL_DOMAIN", true)
}
