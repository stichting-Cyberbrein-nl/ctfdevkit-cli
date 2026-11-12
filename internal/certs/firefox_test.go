package certs

import (
	"encoding/json"
	"reflect"
	"testing"
)

const testCert = "/etc/firefox/policies/devkit-mkcert-rootCA-1000.pem"

func TestAddFirefoxCertificateKeepsExistingPolicies(t *testing.T) {
	existing := []byte(`{
  "policies": {
    "DisableAppUpdate": true,
    "Homepage": {"URL": "https://www.kali.org/?a=1&b=2", "Locked": true},
    "Certificates": {"ImportEnterpriseRoots": true, "Install": ["corp.pem"]},
    "MaxValue": 12345678901234567890
  }
}`)

	updated, changed, err := addFirefoxCertificate(existing, testCert)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}

	var got, want map[string]any
	if err := json.Unmarshal(updated, &got); err != nil {
		t.Fatalf("updated policies are not JSON: %v\n%s", err, updated)
	}
	_ = json.Unmarshal([]byte(`{
  "policies": {
    "DisableAppUpdate": true,
    "Homepage": {"URL": "https://www.kali.org/?a=1&b=2", "Locked": true},
    "Certificates": {"ImportEnterpriseRoots": true, "Install": ["corp.pem", "`+testCert+`"]},
    "MaxValue": 12345678901234567890
  }
}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("policies = %s", updated)
	}

	if again, changed, err := addFirefoxCertificate(updated, testCert); err != nil || changed || string(again) != string(updated) {
		t.Fatalf("second add: changed = %v, err = %v", changed, err)
	}
}

func TestAddFirefoxCertificateCreatesPolicies(t *testing.T) {
	updated, changed, err := addFirefoxCertificate(nil, testCert)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	var got map[string]any
	if err := json.Unmarshal(updated, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"policies": map[string]any{"Certificates": map[string]any{"Install": []any{testCert}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("policies = %s", updated)
	}
}

func TestAddFirefoxCertificateRejectsUnexpectedShape(t *testing.T) {
	for _, doc := range []string{
		`not json`,
		`{"policies": []}`,
		`{"policies": {"Certificates": "x"}}`,
		`{"policies": {"Certificates": {"Install": "x.pem"}}}`,
	} {
		if _, _, err := addFirefoxCertificate([]byte(doc), testCert); err == nil {
			t.Errorf("%s: expected an error instead of overwriting the policy", doc)
		}
	}
}
