package certs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

// firefoxPolicyDir is where every Linux Firefox build reads system policies: Mozilla's deb,
// the Ubuntu snap (through its etc-firefox plug) and Debian/Kali firefox-esr.
const firefoxPolicyDir = "/etc/firefox/policies"

// firefoxDistributionPolicies are the per-install policies.json files Firefox stops reading
// once /etc/firefox/policies/policies.json exists; a new system file starts from them.
var firefoxDistributionPolicies = []string{
	"/usr/lib/firefox/distribution/policies.json",
	"/usr/lib/firefox-esr/distribution/policies.json",
	"/usr/lib64/firefox/distribution/policies.json",
}

// EnsureFirefoxTrust makes every Firefox profile on this Linux machine trust the mkcert CA.
//
// mkcert -install only reaches Firefox profiles that already exist where mkcert looks. On a
// fresh install Firefox has never started, so there is no profile yet, and Firefox 147+
// creates new profiles under ~/.config/mozilla, which mkcert 1.4.x never scans. Because .dev
// is HSTS-preloaded, Firefox then offers no way past the certificate error. The enterprise
// policy Certificates.Install imports the CA into every profile at startup, including
// profiles created later.
func EnsureFirefoxTrust() error {
	if !firefoxInstalled() {
		return nil
	}

	caRoot, err := getMkcertCARoot()
	if err != nil {
		return err
	}
	ca, err := os.ReadFile(filepath.Join(caRoot, "rootCA.pem"))
	if err != nil {
		return fmt.Errorf("reading mkcert CA: %w", err)
	}

	// The snap cannot read hidden directories in $HOME, so the CA is copied next to the
	// policy. One file per user keeps the CAs of users sharing a machine apart.
	certPath := path.Join(firefoxPolicyDir, fmt.Sprintf("devkit-mkcert-rootCA-%d.pem", os.Getuid()))
	policyPath := path.Join(firefoxPolicyDir, "policies.json")

	current, source, err := readFirefoxPolicies(policyPath)
	if err != nil {
		return err
	}
	policies, policyChanged, err := addFirefoxCertificate(current, certPath)
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	installed, _ := os.ReadFile(certPath)
	certChanged := !bytes.Equal(installed, ca)
	if !certChanged && !policyChanged {
		return nil
	}

	output.Info("Configuring Firefox to trust the DevKit certificate...")
	// The certificate goes first so the policy never names a missing file.
	if certChanged {
		if err := installSystemFile(ca, certPath); err != nil {
			return err
		}
	}
	if policyChanged {
		if err := installSystemFile(policies, policyPath); err != nil {
			return err
		}
	}
	output.Success("Firefox trusts the DevKit certificate")
	output.Hint("If Firefox is open, restart it to load the certificate.")
	return nil
}

func firefoxInstalled() bool {
	for _, bin := range []string{"firefox", "firefox-esr"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	_, err := os.Stat("/snap/firefox")
	return err == nil
}

// readFirefoxPolicies returns the system policies.json, or the distribution one it would
// shadow when it does not exist yet, along with the path it came from.
func readFirefoxPolicies(policyPath string) ([]byte, string, error) {
	data, err := os.ReadFile(policyPath)
	if err == nil {
		return data, policyPath, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("reading %s: %w", policyPath, err)
	}
	for _, p := range firefoxDistributionPolicies {
		if data, err := os.ReadFile(p); err == nil {
			return data, p, nil
		}
	}
	return nil, policyPath, nil
}

// addFirefoxCertificate adds certPath to Certificates.Install in a policies.json document,
// keeping every other policy. changed is false when certPath is already listed.
func addFirefoxCertificate(current []byte, certPath string) (updated []byte, changed bool, err error) {
	doc := map[string]any{}
	if len(bytes.TrimSpace(current)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(current))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			return nil, false, err
		}
	}

	policies, err := jsonObject(doc, "policies")
	if err != nil {
		return nil, false, err
	}
	certificates, err := jsonObject(policies, "Certificates")
	if err != nil {
		return nil, false, err
	}
	var install []any
	switch v := certificates["Install"].(type) {
	case nil:
	case []any:
		install = v
	default:
		return nil, false, fmt.Errorf(`"Install" is not a JSON array`)
	}
	if slices.Contains(install, any(certPath)) {
		return current, false, nil
	}
	certificates["Install"] = append(install, certPath)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

// jsonObject returns parent[key] as an object, creating it when absent.
func jsonObject(parent map[string]any, key string) (map[string]any, error) {
	switch v := parent[key].(type) {
	case nil:
		obj := map[string]any{}
		parent[key] = obj
		return obj, nil
	case map[string]any:
		return v, nil
	default:
		return nil, fmt.Errorf("%q is not a JSON object", key)
	}
}

// installSystemFile writes content to a root-owned path, creating its directory, and
// elevates with sudo unless already root.
func installSystemFile(content []byte, dst string) error {
	tmp, err := os.CreateTemp("", "devkit-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	args := []string{"install", "-D", "-m", "0644", tmp.Name(), dst}
	if os.Geteuid() != 0 {
		args = append([]string{"sudo"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing %s: %w", dst, err)
	}
	return nil
}
