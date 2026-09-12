package settings

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

func TestCatalogHasPhaseOneKeys(t *testing.T) {
	required := []string{
		"platform.host", "platform.tls-mode", "platform.additional-ca", "platform.custom-tls",
		"letsencrypt.email", "letsencrypt.solver",
		"storage.replicas", "storage.network", "migration.network", "migration.parallel",
		"overcommit.cpu", "overcommit.memory",
		"backup.target", "backup.credentials-secret", "backup.etcd-schedule",
		"audit.level", "loki.retention-days", "kata.enabled", "loki.enabled",
		"images.refresh-schedule", "images.keep", "authn.require-second-factor",
	}
	for _, key := range required {
		if _, ok := Lookup(key); !ok {
			t.Fatalf("missing setting %q", key)
		}
	}
}

func TestCatalogKeysUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, def := range Catalog() {
		if seen[def.Key] {
			t.Fatalf("duplicate key %q", def.Key)
		}
		seen[def.Key] = true
	}
}

func TestValidateTLSMode(t *testing.T) {
	def, _ := Lookup("platform.tls-mode")
	if err := def.Validate("LetsEncrypt"); err != nil {
		t.Fatal(err)
	}
	if err := def.Validate("Plain"); err == nil {
		t.Fatal("Plain must be invalid")
	}
}

func TestValidateReplicas(t *testing.T) {
	def, _ := Lookup("storage.replicas")
	if err := def.Validate("3"); err != nil {
		t.Fatal(err)
	}
	if err := def.Validate("0"); err == nil {
		t.Fatal("0 must be invalid")
	}
}

func TestCatalogKeysAreValidObjectNames(t *testing.T) {
	for _, def := range Catalog() {
		if errs := validation.IsDNS1123Subdomain(def.Key); len(errs) != 0 {
			t.Fatalf("key %q is not a valid object name: %v", def.Key, errs)
		}
	}
}
