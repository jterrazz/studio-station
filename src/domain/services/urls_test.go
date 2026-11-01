package services

import "testing"

func TestURLsHaveOneShapePerMode(t *testing.T) {
	yes, no := true, false
	l := Lease{VM: "api", GuestPort: 8081, HostPort: 40000, HTTP: &yes}
	if u := URLs(l, "macbook.ts.net", HTTPSPort); u.HTTPS != "https://macbook.ts.net:40000" || u.Local != "http://127.0.0.1:40000" || u.Public() != u.HTTPS {
		t.Fatalf("port mode %+v", u)
	}
	if u := URLs(l, "macbook.ts.net", HTTPSPath); u.HTTPS != "https://macbook.ts.net/api/8081/" {
		t.Fatalf("path mode %+v", u)
	}
	if u := URLs(l, "macbook.ts.net", HTTPSOff); u.HTTPS != "" || u.Public() != "http://macbook.ts.net:40000" {
		t.Fatalf("off %+v", u)
	}
	l.HTTP = &no
	if u := URLs(l, "macbook.ts.net", HTTPSPort); u.HTTPS != "" || u.Local != "127.0.0.1:40000" || u.Public() != "macbook.ts.net:40000" {
		t.Fatalf("tcp %+v", u)
	}
	if Dialed("0.0.0.0") != "127.0.0.1" || Dialed("::") != "::1" || Dialed("192.168.64.7") != "192.168.64.7" {
		t.Fatal("a wildcard is not mapped to its loopback")
	}
}
