package pw

import "testing"

func TestHostingPortUsesServerlessAdapterAssignments(t *testing.T) {
	tests := []struct {
		name     string
		environ  map[string]string
		wantPort int
	}{
		{name: "ordinary server", wantPort: 8080},
		{name: "lambda web adapter", environ: map[string]string{"AWS_LWA_PORT": "9001"}, wantPort: 9001},
		{name: "azure custom handler", environ: map[string]string{"FUNCTIONS_CUSTOMHANDLER_PORT": "43210"}, wantPort: 43210},
		{
			name: "azure assignment is authoritative",
			environ: map[string]string{
				"FUNCTIONS_CUSTOMHANDLER_PORT": "43210",
				"AWS_LWA_PORT":                 "9001",
			},
			wantPort: 43210,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookup := func(name string) (string, bool) {
				value, ok := test.environ[name]
				return value, ok
			}
			got, err := hostingPort(8080, lookup)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.wantPort {
				t.Fatalf("hostingPort = %d, want %d", got, test.wantPort)
			}
		})
	}
}

func TestHostingPortRejectsInvalidAssignments(t *testing.T) {
	for _, value := range []string{"abc", "0", "65536", " 8080"} {
		t.Run(value, func(t *testing.T) {
			_, err := hostingPort(8080, func(string) (string, bool) { return value, true })
			if err == nil {
				t.Fatal("invalid serverless port was accepted")
			}
		})
	}
}
