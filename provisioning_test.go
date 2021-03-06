package fdeutil

import (
	"testing"

	"github.com/chrisccoulson/go-tpm2"
)

func TestProvisionTPM(t *testing.T) {
	tpm, tcti := openTPMSimulatorForTesting(t)
	defer closeTPM(t, tpm)

	resetTPMSimulator(t, tpm, tcti)
	clearTPMWithPlatformAuth(t, tpm)

	lockoutAuth := []byte("1234")

	if err := ProvisionTPM(tpm, lockoutAuth); err != nil {
		t.Fatalf("ProvisionTPM failed: %v", err)
	}

	srkContext, err := tpm.WrapHandle(srkHandle)
	if err != nil {
		t.Errorf("Cannot create context for SRK: %v", err)
	}

	// Validate the properties of the SRK
	pub, _, _, err := tpm.ReadPublic(srkContext)
	if err != nil {
		t.Fatalf("ReadPublic failed: %v", err)
	}

	if pub.Type != tpm2.AlgorithmRSA {
		t.Errorf("SRK has unexpected type")
	}
	if pub.NameAlg != tpm2.AlgorithmSHA256 {
		t.Errorf("SRK has unexpected name algorithm")
	}
	if pub.Attrs != tpm2.AttrFixedTPM|tpm2.AttrFixedParent|tpm2.AttrSensitiveDataOrigin|
		tpm2.AttrUserWithAuth|tpm2.AttrRestricted|tpm2.AttrDecrypt {
		t.Errorf("SRK has unexpected attributes")
	}
	if pub.Params.RSADetail == nil {
		t.Fatalf("SRK public part has no RSA params")
	}
	if pub.Params.RSADetail.Symmetric.Algorithm != tpm2.AlgorithmAES {
		t.Errorf("SRK has unexpected symmetric algorithm")
	}
	if pub.Params.RSADetail.Symmetric.KeyBits.Sym != 128 {
		t.Errorf("SRK has unexpected symmetric key length")
	}
	if pub.Params.RSADetail.Symmetric.Mode.Sym != tpm2.AlgorithmCFB {
		t.Errorf("SRK has unexpected symmetric mode")
	}
	if pub.Params.RSADetail.Scheme.Scheme != tpm2.AlgorithmNull {
		t.Errorf("SRK has unexpected RSA scheme")
	}
	if pub.Params.RSADetail.KeyBits != 2048 {
		t.Errorf("SRK has unexpected RSA public modulus length")
	}
	if pub.Params.RSADetail.Exponent != 0 {
		t.Errorf("SRK has an unexpected non-default public exponent")
	}
	if len(pub.Unique.RSA) != 2048/8 {
		t.Errorf("SRK has an unexpected RSA public modulus length")
	}

	// Validate the DA parameters
	props, err := tpm.GetCapabilityTPMProperties(tpm2.PropertyMaxAuthFail, 3)
	if err != nil {
		t.Fatalf("GetCapability failed: %v", err)
	}
	if props[0].Value != uint32(32) || props[1].Value != uint32(7200) || props[2].Value != uint32(86400) {
		t.Errorf("ProvisionTPM didn't set the DA parameters correctly")
	}

	// Verify that owner control is disabled, that the lockout hierarchy auth is set, and no other hierarchy
	// auth is set
	props, err = tpm.GetCapabilityTPMProperties(tpm2.PropertyPermanent, 1)
	if err != nil {
		t.Fatalf("GetCapability failed: %v", err)
	}
	if tpm2.PermanentAttributes(props[0].Value)&tpm2.AttrLockoutAuthSet == 0 {
		t.Errorf("ProvisionTPM didn't set the lockout hierarchy auth")
	}
	if tpm2.PermanentAttributes(props[0].Value)&tpm2.AttrDisableClear == 0 {
		t.Errorf("ProvisionTPM didn't disable owner clear")
	}
	if tpm2.PermanentAttributes(props[0].Value)&(tpm2.AttrOwnerAuthSet|tpm2.AttrEndorsementAuthSet) > 0 {
		t.Errorf("ProvisionTPM returned with authorizations set for owner or endorsement hierarchies")
	}

	// Test the lockout hierarchy auth
	if err := tpm.DictionaryAttackLockReset(tpm2.HandleLockout, lockoutAuth); err != nil {
		t.Errorf("Use of the lockout hierarchy auth failed: %v", err)
	}

	// Make sure ProvisionTPM didn't leak transient objects
	handles, err := tpm.GetCapabilityHandles(tpm2.HandleTypeTransientObject, tpm2.CapabilityMaxHandles)
	if err != nil {
		t.Fatalf("GetCapability failed: %v", err)
	}
	if len(handles) > 0 {
		t.Errorf("ProvisionTPM leaked transient handles")
	}

	handles, err = tpm.GetCapabilityHandles(tpm2.HandleTypeLoadedSession, tpm2.CapabilityMaxHandles)
	if err != nil {
		t.Fatalf("GetCapability failed: %v", err)
	}
	if len(handles) > 0 {
		t.Errorf("ProvisionTPM leaked loaded session handles")
	}
}

func TestProvisionAlreadyProvisioned(t *testing.T) {
	tpm, _ := openTPMSimulatorForTesting(t)
	defer closeTPM(t, tpm)

	clearTPMWithPlatformAuth(t, tpm)
	if err := ProvisionTPM(tpm, nil); err != nil {
		t.Fatalf("ProvisionTPM failed: %v", err)
	}

	err := ProvisionTPM(tpm, nil)
	if err == nil {
		t.Fatalf("ProvisionTPM should return an error when the TPM is already provisioned")
	}
	if err != ErrClearRequiresPPI {
		t.Errorf("Unexpected error returned from ProvisionTPM: %v", err)
	}
}

func TestProvisionStatus(t *testing.T) {
	tpm, _ := openTPMSimulatorForTesting(t)
	defer closeTPM(t, tpm)

	clearTPMWithPlatformAuth(t, tpm)

	status, err := ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	if status != 0 {
		t.Errorf("Unexpected status")
	}

	lockoutAuth := []byte("1234")

	if err := ProvisionTPM(tpm, lockoutAuth); err != nil {
		t.Fatalf("ProvisionTPM failed: %v", err)
	}

	status, err = ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	expected := AttrValidSRK | AttrDAParamsOK | AttrOwnerClearDisabled | AttrLockoutAuthSet
	if status != expected {
		t.Errorf("Unexpected status")
	}

	if err := tpm.HierarchyChangeAuth(tpm2.HandleLockout, nil, lockoutAuth); err != nil {
		t.Errorf("HierarchyChangeAuth failed: %v", err)
	}

	status, err = ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	expected = AttrValidSRK | AttrDAParamsOK | AttrOwnerClearDisabled
	if status != expected {
		t.Errorf("Unexpected status")
	}

	if err := tpm.ClearControl(tpm2.HandlePlatform, false, nil); err != nil {
		t.Errorf("ClearControl failed: %v", err)
	}

	status, err = ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	expected = AttrValidSRK | AttrDAParamsOK
	if status != expected {
		t.Errorf("Unexpected status")
	}

	if err := tpm.DictionaryAttackParameters(tpm2.HandleLockout, 3, 0, 0, nil); err != nil {
		t.Errorf("DictionaryAttackParameters failed: %v", err)
	}

	status, err = ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	expected = AttrValidSRK
	if status != expected {
		t.Errorf("Unexpected status")
	}

	srkContext, err := tpm.WrapHandle(srkHandle)
	if err != nil {
		t.Fatalf("WrapHandle failed: %v", err)
	}

	if _, err := tpm.EvictControl(tpm2.HandleOwner, srkContext, srkContext.Handle(), nil); err != nil {
		t.Errorf("EvictControl failed: %v", err)
	}

	status, err = ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	expected = 0
	if status != expected {
		t.Errorf("Unexpected status")
	}

	primary, _, _, _, _, _, err := tpm.CreatePrimary(tpm2.HandleOwner, nil, &srkTemplate, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePrimary failed: %v", err)
	}
	defer tpm.FlushContext(primary)

	priv, pub, _, _, _, err := tpm.Create(primary, nil, &srkTemplate, nil, nil, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	context, _, err := tpm.Load(primary, priv, pub, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if _, err := tpm.EvictControl(tpm2.HandleOwner, context, srkHandle, nil); err != nil {
		t.Errorf("EvictControl failed: %v", err)
	}

	status, err = ProvisionStatus(tpm)
	if err != nil {
		t.Errorf("ProvisionStatus failed: %v", err)
	}
	expected = 0
	if status != expected {
		t.Errorf("Unexpected status")
	}
}
