package integration_test

import (
	"os"
	"strings"
	"testing"

	"v.io/core/veyron/lib/testutil/v23tests"
	_ "v.io/core/veyron/profiles"
	"v.io/core/veyron2/naming"
)

func profileCommandOutput(t *testing.T, env v23tests.T, profileBin v23tests.TestBinary, expectError bool, command, name, suffix string) string {
	labelArgs := []string{
		command, naming.Join(name, suffix),
	}
	labelCmd := profileBin.Start(labelArgs...)
	out := labelCmd.Output()
	err := labelCmd.Wait(os.Stdout, os.Stderr)
	if err != nil && !expectError {
		t.Fatalf("%s %q failed: %v\n%v", profileBin.Path(), strings.Join(labelArgs, " "), err, out)
	}
	if err == nil && expectError {
		t.Fatalf("%s %q did not fail when it should", profileBin.Path(), strings.Join(labelArgs, " "))
	}
	return strings.TrimSpace(out)
}

func putProfile(t *testing.T, env v23tests.T, profileBin v23tests.TestBinary, name, suffix string) {
	putArgs := []string{
		"put", naming.Join(name, suffix),
	}
	profileBin.Start(putArgs...).WaitOrDie(os.Stdout, os.Stderr)
}

func removeProfile(t *testing.T, env v23tests.T, profileBin v23tests.TestBinary, name, suffix string) {
	removeArgs := []string{
		"remove", naming.Join(name, suffix),
	}
	profileBin.Start(removeArgs...).WaitOrDie(os.Stdout, os.Stderr)
}

func TestProfileRepository(t *testing.T) {
	env := v23tests.New(t)
	defer env.Cleanup()
	v23tests.RunRootMT(env, "--veyron.tcp.address=127.0.0.1:0")

	// Start the profile repository.
	profileRepoName := "test-profile-repo"
	profileRepoStore := env.TempDir()
	args := []string{
		"-name=" + profileRepoName, "-store=" + profileRepoStore,
		"-veyron.tcp.address=127.0.0.1:0",
	}
	env.BuildGoPkg("v.io/core/veyron/services/mgmt/profile/profiled").Start(args...)

	clientBin := env.BuildGoPkg("v.io/core/veyron/tools/profile")

	// Create a profile.
	const profile = "test-profile"
	putProfile(t, env, clientBin, profileRepoName, profile)

	// Retrieve the profile label and check it matches the
	// expected label.
	profileLabel := profileCommandOutput(t, env, clientBin, false, "label", profileRepoName, profile)
	if got, want := profileLabel, "example"; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Retrieve the profile description and check it matches the
	// expected description.
	profileDesc := profileCommandOutput(t, env, clientBin, false, "description", profileRepoName, profile)
	if got, want := profileDesc, "Example profile to test the profile manager implementation."; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Retrieve the profile specification and check it matches the
	// expected specification.
	profileSpec := profileCommandOutput(t, env, clientBin, false, "specification", profileRepoName, profile)
	if got, want := profileSpec, `profile.Specification{Label:"example", Description:"Example profile to test the profile manager implementation.", Arch:"amd64", OS:"linux", Format:"ELF", Libraries:map[profile.Library]struct {}{profile.Library{Name:"foo", MajorVersion:"1", MinorVersion:"0"}:struct {}{}}}`; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Remove the profile.
	removeProfile(t, env, clientBin, profileRepoName, profile)

	// Check that the profile no longer exists.
	profileCommandOutput(t, env, clientBin, true, "label", profileRepoName, profile)
	profileCommandOutput(t, env, clientBin, true, "description", profileRepoName, profile)
	profileCommandOutput(t, env, clientBin, true, "specification", profileRepoName, profile)
}
