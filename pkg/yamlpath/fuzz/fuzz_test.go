package fuzz

import (
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"testing"
)

func FuzzNewPath(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		path, err := yamlpath.NewPath(string(data))
		if err != nil && path != nil {
			t.Fatalf("fuzz test failed with error: %v", err)
		}
	})
}
