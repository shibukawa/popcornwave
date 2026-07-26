package auth

import (
	"net/http/httptest"
	"testing"
)

func TestPatternMatching(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/account", "/account", true},
		{"/account", "/account/", false},
		{"/account", "/account/settings", false},
		{"/users/*/settings", "/users/42/settings", true},
		{"/users/*/settings", "/users//settings", false},
		{"/users/*/settings", "/users/42/43/settings", false},
		{"/admin/**", "/admin", true},
		{"/admin/**", "/admin/users", true},
		{"/admin/**", "/admin/users/42", true},
		{"/admin/**", "/administration", false},
		{"/**", "/anything/at/all", true},
	}
	for _, c := range cases {
		compiled, err := compilePattern(c.pattern)
		if err != nil {
			t.Fatalf("compile %q: %v", c.pattern, err)
		}
		if got := compiled.match(c.path); got != c.want {
			t.Errorf("%q matches %q = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestPatternCompilationRejectsUnsupportedForms(t *testing.T) {
	for _, value := range []string{
		"account", "/admin/**/users", "/users/*id/settings", "/a//b", "/a/../b", "/a?x=1", "/a#b",
	} {
		if _, err := compilePattern(value); err == nil {
			t.Errorf("pattern %q was accepted", value)
		}
	}
}

func TestCanonicalPathRejectsAmbiguousRequests(t *testing.T) {
	ok := httptest.NewRequest("GET", "/admin/users", nil)
	if path, valid := canonicalPath(ok); !valid || path != "/admin/users" {
		t.Fatalf("canonical path = %q valid=%v", path, valid)
	}
	// An encoded separator would let a request match a shorter pattern than
	// the path the router finally dispatches.
	encoded := httptest.NewRequest("GET", "/admin%2Fusers", nil)
	if _, valid := canonicalPath(encoded); valid {
		t.Fatal("encoded separator was accepted")
	}
}

func TestProtectedAppliesExcludePrecedenceAndKeepsAuthPathsPublic(t *testing.T) {
	include, err := compilePatterns([]string{"/admin/**", "/account"})
	if err != nil {
		t.Fatal(err)
	}
	exclude, err := compilePatterns([]string{"/admin/public"})
	if err != nil {
		t.Fatal(err)
	}
	rt := &runtime{
		config: Config{
			LoginPath: "/auth/login", CallbackPath: "/auth/callback", LogoutPath: "/auth/logout",
		},
		include: include,
		exclude: exclude,
	}
	cases := map[string]bool{
		"/admin":          true,
		"/admin/users":    true,
		"/admin/public":   false,
		"/account":        true,
		"/":               false,
		"/auth/login":     false,
		"/auth/callback":  false,
		"/auth/logout":    false,
		"/public/app.css": false,
	}
	for path, want := range cases {
		if got := rt.protected(path); got != want {
			t.Errorf("protected(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestLocalReturnPathRejectsOffSiteTargets(t *testing.T) {
	for _, value := range []string{
		"https://evil.example/", "//evil.example/", "javascript:alert(1)", "/a/../../b", "", "relative",
	} {
		if got := localReturnPath(value); got != "" {
			t.Errorf("localReturnPath(%q) = %q, want empty", value, got)
		}
	}
	if got := localReturnPath("/mypage"); got != "/mypage" {
		t.Errorf("localReturnPath(/mypage) = %q", got)
	}
}
