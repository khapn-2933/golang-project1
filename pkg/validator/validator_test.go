package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestPasswordStrength_Valid(t *testing.T) {
	t.Parallel()

	v := validator.New()
	_ = RegisterCustomValidators(v)

	type P struct {
		Password string `validate:"password_strength"`
	}

	valid := []string{
		"Secure@123",
		"Abc$1234",
		"P@ssw0rd!",
		"MyP@ss9",
	}
	for _, pw := range valid {
		t.Run(pw, func(t *testing.T) {
			t.Parallel()
			if err := v.Struct(P{Password: pw}); err != nil {
				t.Fatalf("expected %q to pass password_strength, got error: %v", pw, err)
			}
		})
	}
}

func TestPasswordStrength_Invalid(t *testing.T) {
	t.Parallel()

	v := validator.New()
	_ = RegisterCustomValidators(v)

	type P struct {
		Password string `validate:"password_strength"`
	}

	invalid := []struct {
		pw   string
		desc string
	}{
		{"alllowercase1!", "no uppercase"},
		{"ALLUPPERCASE1!", "no lowercase"},
		{"NoDigits!", "no digit"},
		{"NoSpecial1a", "no special char"},
		{"", "empty"},
	}
	for _, tc := range invalid {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			if err := v.Struct(P{Password: tc.pw}); err == nil {
				t.Fatalf("expected %q to fail password_strength but it passed", tc.pw)
			}
		})
	}
}

func TestRegisterCustomValidators(t *testing.T) {
	t.Parallel()

	v := validator.New()
	if err := RegisterCustomValidators(v); err != nil {
		t.Fatalf("RegisterCustomValidators() error: %v", err)
	}
}
