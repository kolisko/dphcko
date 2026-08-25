package invoice

import "testing"

func TestParseMoneyAndRounding(t *testing.T) {
	tests := []struct {
		input string
		want  Money
	}{
		{"0", 0}, {"12", 1200}, {"12,3", 1230}, {"12.34", 1234}, {"-1.25", -125},
	}
	for _, tt := range tests {
		got, err := ParseMoney(tt.input)
		if err != nil || got != tt.want {
			t.Errorf("ParseMoney(%q) = %v, %v; chci %v", tt.input, got, err, tt.want)
		}
	}
	for _, invalid := range []string{"", "1.234", ".5", "abc", "99999999999999999.99"} {
		if _, err := ParseMoney(invalid); err == nil {
			t.Errorf("ParseMoney(%q) měla skončit chybou", invalid)
		}
	}
	if got := (Money(1249)).WholeCrowns(); got != 12 {
		t.Fatalf("12.49 zaokrouhleno na %d", got)
	}
	if got := (Money(1250)).WholeCrowns(); got != 13 {
		t.Fatalf("12.50 zaokrouhleno na %d", got)
	}
	if got := (Money(-1250)).WholeCrowns(); got != -13 {
		t.Fatalf("-12.50 zaokrouhleno na %d", got)
	}
}
