package netutil

import (
	"fmt"
	"net"
	"os"
)

func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("failed to get interface addresses: %w", err)
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no non-loopback IPv4 address found")
}

func MustGetLocalIP() string {
	ip, err := GetLocalIP()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect local IP, using 127.0.0.1: %v\n", err)
		return "127.0.0.1"
	}
	return ip
}
