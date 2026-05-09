package agentctlsdk

import "net/url"

func urlEscape(value string) string {
	return url.PathEscape(value)
}
