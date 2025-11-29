package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/NlCKDEV/kubectl-medic/pkg/k8sclient"
	"github.com/NlCKDEV/kubectl-medic/pkg/output"
	"github.com/NlCKDEV/kubectl-medic/pkg/scanner"
)

func main() {
	namespace := flag.String("namespace", "", "Namespace to scan (empty = all)")
	flag.Parse()

	// For now use the fake client.
	// Later you will swap this with a real Kubernetes client.
	client := k8sclient.NewFakeClient()
	s := scanner.NewScanner(client)

	issues, err := s.ScanNamespace(*namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error scanning namespace: %v\n", err)
		os.Exit(1)
	}

	output.PrintIssuesTable(issues)
}
