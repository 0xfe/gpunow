package validate

import "regexp"

var resourceNameRe = regexp.MustCompile(`^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)
var hostnameDomainRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

func IsResourceName(name string) bool {
	return resourceNameRe.MatchString(name)
}

func IsHostnameDomain(domain string) bool {
	return hostnameDomainRe.MatchString(domain)
}
