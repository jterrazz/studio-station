package machines

import "strconv"

// invariant: a name is held by a VM record on any host, a host, an online node or
// a node station did not tag; an offline `tag:station-vm` node with no record is a
// leftover of a VM that is gone, so its name is free again.
func nameFree(name string, records map[string]bool, nodes map[string]Node) bool {
	if records[name] {
		return false
	}
	node, ok := nodes[name]
	return !ok || (!node.Online && node.StationVM)
}

func allocateName(image string, records map[string]bool, nodes map[string]Node) string {
	for n := 1; ; n++ {
		if name := image + "-" + strconv.Itoa(n); nameFree(name, records, nodes) {
			return name
		}
	}
}
