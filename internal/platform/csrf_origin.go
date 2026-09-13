package platform

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var errInvalidPublicOrigin = errors.New("BTG_LMS_PUBLIC_ORIGIN must be an exact http(s) origin without path, query, or fragment")

type origin struct {
	scheme string
	host   string
	port   int
}

func parseOrigin(raw string) (origin, error) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return origin{}, errInvalidPublicOrigin
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Opaque != "" || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Hostname() == "" || u.RawFragment != "" {
		return origin{}, errInvalidPublicOrigin
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return origin{}, errInvalidPublicOrigin
	}
	port := 443
	if scheme == "http" {
		port = 80
	}
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return origin{}, errInvalidPublicOrigin
		}
	}
	return origin{scheme: scheme, host: strings.ToLower(u.Hostname()), port: port}, nil
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type originPolicy struct{ allowed []origin }

func newOriginPolicy(cfg Config) (originPolicy, error) {
	public, err := parseOrigin(cfg.PublicOrigin)
	if err != nil {
		return originPolicy{}, err
	}
	if !cfg.DevelopmentHTTP {
		if public.scheme != "https" {
			return originPolicy{}, errors.New("production public origin must use HTTPS")
		}
		return originPolicy{allowed: []origin{public}}, nil
	}
	if public.scheme != "http" || !loopbackHost(public.host) {
		return originPolicy{}, errors.New("development public origin must use loopback HTTP")
	}
	host, portText, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil || !loopbackHost(host) {
		return originPolicy{}, errors.New("development HTTP must bind to loopback")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return originPolicy{}, errors.New("invalid development HTTP port")
	}
	return originPolicy{allowed: []origin{public, {scheme: "http", host: strings.ToLower(host), port: port}}}, nil
}

func mutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Missing Origin is rejected on mutations. Host/Forwarded/X-Forwarded-*
// headers never contribute to the trusted origin set.
func (p originPolicy) allowsRequest(r *http.Request) bool {
	if !mutatingMethod(r.Method) {
		return true
	}
	values := r.Header.Values("Origin")
	if len(values) != 1 {
		return false
	}
	actual, err := parseOrigin(values[0])
	if err != nil {
		return false
	}
	for _, allowed := range p.allowed {
		if actual == allowed {
			return true
		}
	}
	return false
}
