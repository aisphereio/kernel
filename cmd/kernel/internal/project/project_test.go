package project

import (
	"reflect"
	"testing"
)

func TestResolveLayoutPrefersExplicitRepo(t *testing.T) {
	want := "local-kernel-layout"
	got, err := resolveLayout(want, t.TempDir())
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	if got != want {
		t.Fatalf("expected explicit repo %q, got %q", want, got)
	}
}

func TestResolveLayoutUsesEnvironment(t *testing.T) {
	want := "env-kernel-layout"
	t.Setenv("KERNEL_LAYOUT", want)
	got, err := resolveLayout("", t.TempDir())
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	if got != want {
		t.Fatalf("expected env layout %q, got %q", want, got)
	}
}

func TestResolveLayoutDefaultsToKernelLayoutRepository(t *testing.T) {
	t.Setenv("KERNEL_LAYOUT", "")
	got, err := resolveLayout("", t.TempDir())
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	want := "https://github.com/aisphereio/kernel-layout.git"
	if got != want {
		t.Fatalf("expected default layout repo %q, got %q", want, got)
	}
}

func TestMergeDisabledFeaturesExpandsIAM(t *testing.T) {
	got := mergeDisabledFeatures(nil, []string{"iam"})
	want := []string{"iam", "authn", "authz", "access", "gateway"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestMVPScaffoldOptions(t *testing.T) {
	opts := mvpScaffoldOptions()
	if opts.Profile != layoutProfileMVP {
		t.Fatalf("expected MVP profile, got %q", opts.Profile)
	}
	for _, disabled := range []string{"iam", "authn", "authz", "access", "gateway", "dbx", "cachex", "objectstorex", "dtmx", "auditx", "metricsx"} {
		found := false
		for _, got := range opts.DisabledFeatures {
			if got == disabled {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected disabled feature %q in %v", disabled, opts.DisabledFeatures)
		}
	}
}
