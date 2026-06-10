package llamafirewall

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultPromptGuardModel is the gated 22M PromptGuard 2 model used by default
// (CPU/Windows friendly). It is a GATED Hugging Face model: it requires accepting
// the Llama license on huggingface.co and `huggingface-cli login` before it can
// be fetched (Phase 20, LLMF — human-only web action).
const DefaultPromptGuardModel = "meta-llama/Llama-Prompt-Guard-2-22M"

// Embedded sidecar assets (Phase 20, LLMF). //go:embed requires the assets to
// live under the embedding package, so the old top-level sidecar/ directory was
// moved to internal/llamafirewall/assets/.
//
//go:embed assets/llamafirewall_sidecar.py assets/requirements.txt
var sidecarAssets embed.FS

// SidecarScriptName / SidecarRequirementsName are the on-disk file names written
// under <stateDir>/llamafirewall/ by InstallSidecar.
const (
	SidecarScriptName       = "llamafirewall_sidecar.py"
	SidecarRequirementsName = "requirements.txt"
	sidecarStampName        = ".sidecar.sha256"
)

// SidecarDir returns the directory under stateDir where the sidecar assets and
// venv live: <stateDir>/llamafirewall.
func SidecarDir(stateDir string) string {
	return filepath.Join(stateDir, "llamafirewall")
}

// SidecarScriptPath returns the installed sidecar script path.
func SidecarScriptPath(stateDir string) string {
	return filepath.Join(SidecarDir(stateDir), SidecarScriptName)
}

// VenvDir returns the Python venv directory created by `beekeeper llamafirewall
// install`: <stateDir>/llamafirewall/venv.
func VenvDir(stateDir string) string {
	return filepath.Join(SidecarDir(stateDir), "venv")
}

// VenvPython returns the venv interpreter path for the given venv directory
// (Scripts\python.exe on Windows, bin/python elsewhere).
func VenvPython(venvDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "python.exe")
	}
	return filepath.Join(venvDir, "bin", "python")
}

// HFHome returns the pinned Hugging Face cache dir for the gated model:
// <stateDir>/llamafirewall/hf. Injected as HF_HOME so the model cache lives under
// the StateDir, not the user's default ~/.cache.
func HFHome(stateDir string) string {
	return filepath.Join(SidecarDir(stateDir), "hf")
}

// InstallSidecar writes the embedded sidecar script + requirements.txt under
// <stateDir>/llamafirewall with 0600 perms and a sha256 stamp covering both
// files. A second call whose embedded content hashes to the same stamp is a
// no-op (hash-skip); a stamp mismatch (an upgraded binary) rewrites both files
// and the stamp. Returns the installed script path.
func InstallSidecar(stateDir string) (string, error) {
	dir := SidecarDir(stateDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create sidecar dir %q: %w", dir, err)
	}

	script, err := sidecarAssets.ReadFile("assets/" + SidecarScriptName)
	if err != nil {
		return "", fmt.Errorf("read embedded sidecar script: %w", err)
	}
	reqs, err := sidecarAssets.ReadFile("assets/" + SidecarRequirementsName)
	if err != nil {
		return "", fmt.Errorf("read embedded requirements: %w", err)
	}

	want := sidecarStamp(script, reqs)
	stampPath := filepath.Join(dir, sidecarStampName)
	scriptPath := filepath.Join(dir, SidecarScriptName)

	// Hash-skip: if the stamp matches and the script is present, do nothing.
	if got, rerr := os.ReadFile(stampPath); rerr == nil && string(got) == want {
		if _, serr := os.Stat(scriptPath); serr == nil {
			return scriptPath, nil
		}
	}

	if err := os.WriteFile(scriptPath, script, 0o600); err != nil {
		return "", fmt.Errorf("write sidecar script: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SidecarRequirementsName), reqs, 0o600); err != nil {
		return "", fmt.Errorf("write requirements: %w", err)
	}
	if err := os.WriteFile(stampPath, []byte(want), 0o600); err != nil {
		return "", fmt.Errorf("write sidecar stamp: %w", err)
	}
	return scriptPath, nil
}

// sidecarStamp returns a hex sha256 over the script + requirements content, used
// as the version stamp for hash-skip / rewrite-on-upgrade.
func sidecarStamp(script, reqs []byte) string {
	h := sha256.New()
	h.Write(script)
	h.Write(reqs)
	return hex.EncodeToString(h.Sum(nil))
}
