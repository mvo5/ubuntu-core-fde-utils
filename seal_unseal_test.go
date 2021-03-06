package fdeutil

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestCreateAndUnseal(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	if err := ProvisionTPM(tpm, nil); err != nil && err != ErrClearRequiresPPI {
		t.Fatalf("Failed to provision TPM for test: %v", err)
	}

	status, err := ProvisionStatus(tpm)
	if err != nil {
		t.Fatalf("Cannot check provision status: %v", err)
	}
	if status&AttrValidSRK == 0 {
		t.Fatalf("No valid SRK for test")
	}

	key := make([]byte, 64)
	rand.Read(key)

	tmpDir, err := ioutil.TempDir("", "_TestCreateAndUnseal_")
	if err != nil {
		t.Fatalf("Creating temporary directory failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dest := tmpDir + "/keydata"

	if err := SealKeyToTPM(tpm, dest, Create, key); err != nil {
		t.Fatalf("SealKeyToTPM failed: %v", err)
	}

	f, err := os.Open(dest)
	if err != nil {
		t.Fatalf("Failed to open key data file: %v", err)
	}

	keyUnsealed, err := UnsealKeyFromTPM(tpm, f, "")
	if err != nil {
		t.Fatalf("UnsealKeyFromTPM failed: %v", err)
	}

	if !bytes.Equal(key, keyUnsealed) {
		t.Errorf("TPM returned the wrong key")
	}
}

func TestCreateDoesntReplace(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	if err := ProvisionTPM(tpm, nil); err != nil && err != ErrClearRequiresPPI {
		t.Fatalf("Failed to provision TPM for test: %v", err)
	}

	status, err := ProvisionStatus(tpm)
	if err != nil {
		t.Fatalf("Cannot check provision status: %v", err)
	}
	if status&AttrValidSRK == 0 {
		t.Fatalf("No valid SRK for test")
	}

	key := make([]byte, 64)
	rand.Read(key)

	tmpDir, err := ioutil.TempDir("", "_TestCreateDoesntReplace_")
	if err != nil {
		t.Fatalf("Creating temporary directory failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dest := tmpDir + "/keydata"

	if err := SealKeyToTPM(tpm, dest, Create, key); err != nil {
		t.Fatalf("SealKeyToTPM failed: %v", err)
	}

	fi1, err := os.Stat(dest)
	if err != nil {
		t.Errorf("Cannot stat key data file: %v", err)
	}

	err = SealKeyToTPM(tpm, dest, Create, key)
	if err == nil {
		t.Fatalf("SealKeyToTPM Create should fail if there is already a file with the same path")
	}
	if err.Error() != "cannot create new key data file: file already exists" {
		t.Errorf("Unexpected error: %v", err)
	}

	fi2, err := os.Stat(dest)
	if err != nil {
		t.Errorf("Cannot stat key data file: %v", err)
	}

	if fi1.ModTime() != fi2.ModTime() {
		t.Errorf("SealKeyToTPM Create modified the existing file")
	}
}

func TestUpdateAndUnseal(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	if err := ProvisionTPM(tpm, nil); err != nil && err != ErrClearRequiresPPI {
		t.Fatalf("Failed to provision TPM for test: %v", err)
	}

	status, err := ProvisionStatus(tpm)
	if err != nil {
		t.Fatalf("Cannot check provision status: %v", err)
	}
	if status&AttrValidSRK == 0 {
		t.Fatalf("No valid SRK for test")
	}

	key := make([]byte, 64)
	rand.Read(key)

	tmpDir, err := ioutil.TempDir("", "_TestUpdateAndUnseal_")
	if err != nil {
		t.Fatalf("Creating temporary directory failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dest := tmpDir + "/keydata"

	if err := SealKeyToTPM(tpm, dest, Create, key); err != nil {
		t.Fatalf("SealKeyToTPM failed: %v", err)
	}

	testPIN := "1234"

	if err := ChangePIN(tpm, dest, "", testPIN); err != nil {
		t.Fatalf("ChangePIN failed: %v", err)
	}

	fi1, err := os.Stat(dest)
	if err != nil {
		t.Errorf("Cannot stat key data file: %v", err)
	}

	if err := SealKeyToTPM(tpm, dest, Update, key); err != nil {
		t.Fatalf("SealKeyToTPM failed: %v", err)
	}

	fi2, err := os.Stat(dest)
	if err != nil {
		t.Errorf("Cannot stat key data file: %v", err)
	}

	if fi1.ModTime() == fi2.ModTime() {
		t.Errorf("File wasn't updated")
	}

	f, err := os.Open(dest)
	if err != nil {
		t.Fatalf("Failed to open key data file: %v", err)
	}

	keyUnsealed, err := UnsealKeyFromTPM(tpm, f, testPIN)
	if err != nil {
		t.Fatalf("UnsealKeyFromTPM failed: %v", err)
	}

	if !bytes.Equal(key, keyUnsealed) {
		t.Errorf("TPM returned the wrong key")
	}
}

func TestUpdateWithoutExisting(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	if err := ProvisionTPM(tpm, nil); err != nil && err != ErrClearRequiresPPI {
		t.Fatalf("Failed to provision TPM for test: %v", err)
	}

	status, err := ProvisionStatus(tpm)
	if err != nil {
		t.Fatalf("Cannot check provision status: %v", err)
	}
	if status&AttrValidSRK == 0 {
		t.Fatalf("No valid SRK for test")
	}

	key := make([]byte, 64)
	rand.Read(key)

	tmpDir, err := ioutil.TempDir("", "_TestUpdateWithoutExisting_")
	if err != nil {
		t.Fatalf("Creating temporary directory failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dest := tmpDir + "/keydata"

	err = SealKeyToTPM(tpm, dest, Update, key)
	if err == nil {
		t.Fatalf("SealKeyToTPM Update should fail if there isn't a valid key data file")
	}
	if !strings.HasPrefix(err.Error(), "cannot open existing key data file to update: ") {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, err := os.Stat(dest); err == nil || !os.IsNotExist(err) {
		t.Errorf("SealKeyToTPM Update should not create a file where there isn't one")
	}
}
