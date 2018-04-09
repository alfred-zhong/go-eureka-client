package eureka

import (
	"testing"
)

func Test_NewInstance(t *testing.T) {
	c1, err := NewInstance("testapp")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(c1)

	_, err = NewInstance("")
	if err == nil {
		t.Fatal()
	}
}

func Test_NewInstanceWithPort(t *testing.T) {
	c1, err := NewInstanceWithPort("testapp", 9000)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(c1)
}
