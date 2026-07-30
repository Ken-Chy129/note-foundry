package sources

import (
	"net"
	"net/url"
	"sort"
	"strings"
)

func NormalizeSourceURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Opaque != "" || parsed.User != nil {
		return "", ErrSourceURLInvalid
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrSourceURLInvalid
	}
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" {
		return "", ErrSourceURLInvalid
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(hostname, ":") {
		if port == "" {
			parsed.Host = "[" + hostname + "]"
		} else {
			parsed.Host = net.JoinHostPort(hostname, port)
		}
	} else if port == "" {
		parsed.Host = hostname
	} else {
		parsed.Host = net.JoinHostPort(hostname, port)
	}
	parsed.User = nil
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.ForceQuery = false
	parsed.RawPath = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	query := parsed.Query()
	for key := range query {
		sort.Strings(query[key])
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func defaultURLSourceTitle(normalizedURL string) string {
	parsed, err := url.Parse(normalizedURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
