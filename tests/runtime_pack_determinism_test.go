package cli_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRuntimePackCanonicalizesInventoryLineEndings(t *testing.T) {
	binary := buildRuntimePack(t)
	var noticeDigests []string

	for _, inventory := range []string{
		"example.com/dependency,https://example.com/LICENSE,MIT\n",
		"example.com/dependency,https://example.com/LICENSE,MIT\r\n",
	} {
		workspace := t.TempDir()
		artifact := filepath.Join(workspace, "runtime.zip")
		writeZip(t, artifact, map[string]string{"license.txt": "PHP license\n"})
		writeFile(t, filepath.Join(workspace, "go-licenses.csv"), inventory)
		writeFile(t, filepath.Join(workspace, "THIRD-PARTY-NOTICES.md"), "# Notices\n")
		writeFile(t, filepath.Join(workspace, "licenses", "example.com", "dependency", "LICENSE"), "MIT License\n")
		writeFile(
			t,
			filepath.Join(workspace, "recipe.json"),
			runtimeRecipe(
				fileSHA256(t, artifact),
				`"THIRD-PARTY-NOTICES.md", "licenses/example.com/dependency/LICENSE"`,
				`"license.txt"`,
			),
		)

		output := filepath.Join(workspace, "dist")
		command := exec.Command(binary, "assemble", "--recipe", filepath.Join(workspace, "recipe.json"), "--artifact", artifact, "--output", output)
		if combined, err := command.CombinedOutput(); err != nil {
			t.Fatalf("assemble runtime with inventory %q: %v\n%s", inventory, err, combined)
		}
		noticeDigests = append(noticeDigests, fileSHA256(t, filepath.Join(output, "third-party-notices-windows-x64.zip")))
	}

	if noticeDigests[0] != noticeDigests[1] {
		t.Fatalf("notice bundle depends on inventory line endings: LF=%s CRLF=%s", noticeDigests[0], noticeDigests[1])
	}
}
