package eureka

import (
	"net"
	"strings"

	"github.com/pkg/errors"
)

// AutoIP4 自动获取适当的 IP4
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

		// TODO: 处理其他 interface 的 IP
	}

	return "", false
}

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
