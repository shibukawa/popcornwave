package auth

import (
	"testing"

	"github.com/shibukawa/popcornwave/internal/pathpattern"
)

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

func TestProtectedAppliesExcludePrecedenceAndKeepsAuthPathsPublic(t *testing.T) {
	include, err := pathpattern.Compile([]string{"/admin/**", "/account"})
	if err != nil {
		t.Fatal(err)
	}
	exclude, err := pathpattern.Compile([]string{"/admin/public"})
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
