package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chrisccoulson/ubuntu-core-fde-utils"
)

var (
	requestClear bool
)

func init() {
	flag.BoolVar(&requestClear, "request-clear", false, "")
}

func main() {
	flag.Parse()

	args := flag.Args()

	if requestClear {
		if err := fdeutil.RequestTPMClearUsingPPI(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to request clearing the TPM via the PPI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var lockoutAuth string
	if len(args) > 0 {
		lockoutAuth = args[0]
	}

	tpm, err := fdeutil.ConnectToDefaultTPM()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot acquire TPM context: %v", err)
		os.Exit(1)
	}
	defer tpm.Close()

	if err := fdeutil.ProvisionTPM(tpm, []byte(lockoutAuth)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to provision the TPM: %v\n", err)
		os.Exit(1)
	}
}
