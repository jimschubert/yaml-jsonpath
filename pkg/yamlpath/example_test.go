/*
 * Copyright 2020 VMware, Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package yamlpath_test

import (
	"bytes"
	"fmt"
	"log"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"go.yaml.in/yaml/v3"
)

// Example uses a Path to find certain nodes and replace their content. Unlike a global change, it avoids false positives.
func Example() {
	y := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-deployment
spec:
  template:
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
      - name: nginy
        image: nginy
        ports:
        - containerPort: 81
`
	var n yaml.Node

	err := yaml.Unmarshal([]byte(y), &n)
	if err != nil {
		log.Fatalf("cannot unmarshal data: %v", err)
	}

	p, err := yamlpath.NewPath("$..spec.containers[*].image")
	if err != nil {
		log.Fatalf("cannot create path: %v", err)
	}

	q, err := p.Find(&n)
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}

	for _, i := range q {
		i.Value = "example.com/user/" + i.Value
	}

	var buf bytes.Buffer
	e := yaml.NewEncoder(&buf)
	defer e.Close()
	e.SetIndent(2)

	err = e.Encode(&n)
	if err != nil {
		log.Printf("Error: cannot marshal node: %v", err)
		return
	}

	z := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-deployment
spec:
  template:
    spec:
      containers:
        - name: nginx
          image: example.com/user/nginx
          ports:
            - containerPort: 80
        - name: nginy
          image: example.com/user/nginy
          ports:
            - containerPort: 81
`
	if buf.String() == z {
		fmt.Printf("success")
	} else {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(buf.String(), z, false)
		fmt.Println(diffs)
	}

	// Output: success
}
