package pwenv

import "testing"

// "dev" is two different facts wearing one name: a deployment that asked for the
// development relaxations, and one that forgot to say anything. The relaxations
// key off the first, so the two have to be distinguishable here.
func TestAnUnsetEnvironmentDefaultsWithoutDeclaring(t *testing.T) {
	value, declared, err := ResolveDeclared([]string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatal(err)
	}
	if value != Development {
		t.Errorf("value = %q, want %q so the config file still resolves", value, Development)
	}
	if declared {
		t.Error("an unset APP_ENV must not count as declaring development")
	}
}

func TestAnEmptyEnvironmentDefaultsWithoutDeclaring(t *testing.T) {
	_, declared, err := ResolveDeclared([]string{Var + "=   "})
	if err != nil {
		t.Fatal(err)
	}
	if declared {
		t.Error("a blank APP_ENV must not count as declaring development")
	}
}

func TestANamedEnvironmentDeclares(t *testing.T) {
	for _, want := range []string{Development, Staging, Production, "qa"} {
		value, declared, err := ResolveDeclared([]string{Var + "=" + want})
		if err != nil {
			t.Fatalf("%s: %v", want, err)
		}
		if value != want {
			t.Errorf("value = %q, want %q", value, want)
		}
		if !declared {
			t.Errorf("%s: a named environment must declare", want)
		}
	}
}

// An invalid token still fails, and reports nothing as declared, so a caller
// cannot read a relaxation out of a value that never resolved.
func TestAnInvalidEnvironmentDeclaresNothing(t *testing.T) {
	_, declared, err := ResolveDeclared([]string{Var + "=../etc"})
	if err == nil {
		t.Fatal("an environment containing a path separator must fail")
	}
	if declared {
		t.Error("an invalid environment must not declare")
	}
}
