package fail_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/core/fail"
)

func TestAnEmptyHintPanicsUnderTest(t *testing.T) {
	for name, build := range map[string]func(){
		"usage":  func() { _ = fail.Usage("unknown tool \"nope\"", "") },
		"failed": func() { _ = fail.Failed("install failed", "", nil) },
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
