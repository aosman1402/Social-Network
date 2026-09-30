package handlers

import "testing"

func TestIsValidEmail(t *testing.T) {
	cases := []struct {
		email string
		want  bool
	}{
		{"a@b.com", true},
		{"first.last@sub.example.co", true},
		{"no-at-sign.com", false},
		{"@no-local-part.com", true}, // format check only, not full RFC 5322
		{"trailing-at@", false},
		{"no-dot@domain", false},
		{"", false},
		{"a@b", false},
	}

	for _, c := range cases {
		if got := isValidEmail(c.email); got != c.want {
			t.Errorf("isValidEmail(%q) = %v, want %v", c.email, got, c.want)
		}
	}
}

func TestIsValidPassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		want     bool
	}{
		{"valid 12 chars all categories", "Sup3rSec9!zx", true},
		{"16 chars, over max", "Sup3rSecure9!zxy", false},
		{"exactly 15 chars (max)", "Sup3rSecure9!zx", true},
		{"11 chars, under min", "Sup3rSec9!z", false},
		{"missing uppercase", "sup3rsec9!zx", false},
		{"missing lowercase", "SUP3RSEC9!ZX", false},
		{"missing digit", "SuperSecure!zx", false},
		{"missing special char", "Sup3rSecure9zx", false},
		{"disallowed character", "Sup3rSec9!zxé", false}, // é is outside every allowed category
		{"empty", "", false},
		{"all spaces special char counts, but nothing else", "            ", false},
	}

	for _, c := range cases {
		if got := isValidPassword(c.password); got != c.want {
			t.Errorf("%s: isValidPassword(%q) = %v, want %v", c.name, c.password, got, c.want)
		}
	}
}

func TestIsValidNicknameFormat(t *testing.T) {
	cases := []struct {
		nickname string
		want     bool
	}{
		{"abc", true}, // exactly 3 (min)
		{"Abc123", true},
		{"fourteenChars1", true},   // exactly 14 (max)
		{"fifteenChars123", false}, // exactly 15, over max
		{"ab", false},              // exactly 2, under min
		{"", false},
		{"has space", false},
		{"has-dash", false},
		{"has_underscore", false},
	}

	for _, c := range cases {
		if got := isValidNicknameFormat(c.nickname); got != c.want {
			t.Errorf("isValidNicknameFormat(%q) = %v, want %v", c.nickname, got, c.want)
		}
	}
}
