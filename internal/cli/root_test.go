package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/neilfarmer/k8s-health/internal/cli"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRootHelp(t *testing.T) {
	t.Parallel()
	out, err := execute(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"khealth", "check", "test", "version"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q\n----\n%s", want, out)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()
	out, err := execute(t, "version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "khealth") {
		t.Errorf("version output missing binary name: %q", out)
	}
}

func TestVersionJSON(t *testing.T) {
	t.Parallel()
	out, err := execute(t, "version", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"version"`) {
		t.Errorf("expected JSON with version field, got: %q", out)
	}
}

func TestCheckSubcommandsRegistered(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"cluster", "pods", "nodes", "controlplane", "etcd", "events"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			out, err := execute(t, "check", scope, "--help")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, scope) {
				t.Errorf("help did not mention scope %q: %s", scope, out)
			}
		})
	}
}

func TestCheckClusterErrorsWithoutKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", "/nonexistent/no/such/kubeconfig.yaml")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	out, err := execute(t,
		"check", "cluster",
		"--launch-mode", "out-of-cluster",
		"--kubeconfig", "/nonexistent/no/such/kubeconfig.yaml",
	)
	if err == nil {
		t.Fatalf("expected error when kubeconfig missing, output=%s", out)
	}
}

func TestTestSubcommandStubs(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"run", "lint", "list"} {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			out, err := execute(t, "test", v, "./tests")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "not yet implemented") {
				t.Errorf("expected stub message for test %s, got: %q", v, out)
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	_, err := execute(t, "wat")
	if err == nil {
		t.Fatal("expected an error for unknown command")
	}
}
