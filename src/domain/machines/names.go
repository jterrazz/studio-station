package machines

import "strconv"

// invariant: a name is held by a VM record on any host of the fleet or by a
// host; no tailnet node holds a VM's name since VMs left the tailnet.
func nameFree(name string, held map[string]bool) bool { return !held[name] }

func allocateName(image string, held map[string]bool) string {
	for n := 1; ; n++ {
		if name := image + "-" + strconv.Itoa(n); nameFree(name, held) {
			return name
		}
	}
}
