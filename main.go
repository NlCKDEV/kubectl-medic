package main

import (
	"os"

	"github.com/NlCKDEV/kubectl-medic/cmd/medic"
)

func main() {
	if err := medic.Execute(); err != nil {
		os.Exit(1)
	}
}
