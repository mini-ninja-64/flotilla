package util

import "fmt"

const (
	HTTP  = "http"
	HTTPS = "https"
)

func GetDefaultPortForProtocol(protocol *string) (uint16, error) {
	switch *protocol {
	case HTTP:
		return 80, nil
	case HTTPS:
		return 443, nil
	default:
		return 0, fmt.Errorf("Unsure of default port for protocol '%s'", *protocol)
	}
}
