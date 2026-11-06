package calls

import "strconv"

type claim int

const (
	unclaimed claim = iota
	claimMac
	claimVM
)

// invariant: a name is held by a VM record on any host of the fleet or by a
// host; no tailnet node holds a VM's name since VMs left the tailnet.
func nameFree(name string, held map[string]claim) bool { return held[name] == unclaimed }

func allocateName(image string, held map[string]claim) string {
	for n := 1; ; n++ {
		if name := image + "-" + strconv.Itoa(n); nameFree(name, held) {
			return name
		}
	}
}
