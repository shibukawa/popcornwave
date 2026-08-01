package dynamo

import (
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	config := DefaultConfig()
	config.Enabled = true
	config.Region = "ap-northeast-1"
	return config
}

func TestValidateAcceptsADisabledSection(t *testing.T) {
	// A disabled section is not a half-configured one, so nothing about it is
	// worth reporting.
	if err := (Config{}).validate(false); err != nil {
		t.Fatalf("a disabled section must validate: %v", err)
	}
}

func TestValidateRejectsHalfConfiguredCredentials(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"key without secret", func(c *Config) { c.AccessKeyID = "AKIA" }, "secret_access_key"},
		{"secret without key", func(c *Config) { c.SecretAccessKey = "s" }, "access_key_id"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := validConfig()
			testCase.mutate(&config)
			err := config.validate(false)
			if err == nil {
				t.Fatal("half-configured credentials must be rejected")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error must name the missing key, got %v", err)
			}
			// The values themselves must never reach the message.
			if strings.Contains(err.Error(), "AKIA") {
				t.Fatalf("error leaked a credential: %v", err)
			}
		})
	}
}

func TestValidateRejectsAutoMigrateOutsideDevelopment(t *testing.T) {
	config := validConfig()
	config.AutoMigrate = true
	if err := config.validate(true); err != nil {
		t.Fatalf("auto_migrate must be allowed in development: %v", err)
	}
	err := config.validate(false)
	if err == nil {
		t.Fatal("auto_migrate outside development must be a configuration error, not a warning")
	}
	if !strings.Contains(err.Error(), "auto_migrate") {
		t.Fatalf("error must name the key, got %v", err)
	}
}

func TestValidateRejectsANonPositiveTimeout(t *testing.T) {
	config := validConfig()
	config.Timeout = 0
	if err := config.validate(false); err == nil {
		t.Fatal("a zero timeout must be rejected")
	}
	config.Timeout = -time.Second
	if err := config.validate(false); err == nil {
		t.Fatal("a negative timeout must be rejected")
	}
}

func TestValidateRejectsADuplicateTableMapping(t *testing.T) {
	config := validConfig()
	config.TableNames = []TableName{
		{Declared: "reading", Deployed: "a-reading"},
		{Declared: "reading", Deployed: "b-reading"},
	}
	err := config.validate(false)
	if err == nil {
		t.Fatal("one declared table cannot map to two deployed names")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Fatalf("error must say what is wrong, got %v", err)
	}
}

func TestValidateRejectsAnIllegalPrefix(t *testing.T) {
	config := validConfig()
	config.TablePrefix = "not ok/"
	if err := config.validate(false); err == nil {
		t.Fatal("a prefix outside the DynamoDB character set must be rejected")
	}
}

func TestDefaultConfigVerifiesSchema(t *testing.T) {
	// Verification is the production value of this package, so it is the
	// default rather than something a deployment has to remember.
	if !DefaultConfig().VerifySchema {
		t.Fatal("verify_schema must default on")
	}
	if DefaultConfig().AutoMigrate {
		t.Fatal("auto_migrate must default off")
	}
}
