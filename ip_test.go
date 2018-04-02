package eureka

import (
	"testing"
)

func Test_autoIPs(t *testing.T) {
	ipsMap, err := allIPs()
	if err != nil {
		t.Fatal(err)
	}
	for iname, ips := range ipsMap {
		t.Logf("%s - %v", iname, ips)
	}
}

func Test_AutoIP4(t *testing.T) {
	ip, ok := AutoIP4()
	t.Log(ip, ok)
}
