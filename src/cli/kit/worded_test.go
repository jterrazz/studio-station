package kit

import "testing"

func TestAnEmptyHintPanicsUnderTest(t *testing.T) {
	for name, build := range map[string]func(){
		"usage":  func() { _ = usage("unknown tool \"nope\"", "") },
		"failed": func() { _ = failed("install failed", "", nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("an error built with an empty hint did not panic")
				}
			}()
			build()
		})
	}
}
