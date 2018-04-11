package eureka

import (
	"net"
	"strings"

	"github.com/pkg/errors"
)

// AutoIP4 fetch the proper IPv4 address.
//
// Still not stable and reliable.
func AutoIP4() (string, bool) {
	ipsMap, err := allIPs()
	if err != nil {
		return "", false
	}

	for interfceName, ips := range ipsMap {
		if strings.HasPrefix(interfceName, "en") {
			for _, ip := range ips {
				if _, err := net.ResolveIPAddr("ip4", ip); err == nil {
					return ip, true
				}
			}
		}

		// TODO: Handle ip from other interface
	}

	return "", false
}

// allIPs returns map including ip list mapping to their interface. Map key is
// interface name. Map value is the ip list belongs to that interface.
func allIPs() (map[string][]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err, "net.Interfaces()")
	}

	ipsMap := make(map[string][]string)
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		ips := make([]string, 0)
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil {
				ips = append(ips, ip.String())
			}
		}

		if len(ips) > 0 {
			ipsMap[i.Name] = ips
		}
	}

	return ipsMap, nil
}
