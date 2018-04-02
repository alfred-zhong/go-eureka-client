package eureka

import (
	"testing"
)

func Test_NewClient(t *testing.T) {
	c1, err := NewClient("testapp")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(c1)

	_, err = NewClient("")
	if err == nil {
		t.Fatal()
	}
}

func Test_NewClientWithPort(t *testing.T) {
	c1, err := NewClientWithPort("testapp", 9000)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(c1)
}
