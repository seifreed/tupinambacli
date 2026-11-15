package cookieheader

import "testing"

func TestFormatSortsCookieNames(t *testing.T) {
	got := Format(map[string]string{"z": "last", "a": "first", "m": "middle"})
	if got != "a=first; m=middle; z=last" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestFormatEmptyCookies(t *testing.T) {
	if got := Format(nil); got != "" {
		t.Fatalf("Format(nil) = %q", got)
	}
}

func TestParseCookieHeader(t *testing.T) {
	got := Parse(" cart=old ; invalid ; =ignored; keep=1; cart=new; equals=a=b ")
	if len(got) != 3 || got["cart"] != "new" || got["keep"] != "1" || got["equals"] != "a=b" {
		t.Fatalf("Parse() = %#v", got)
	}
	if got := Parse(""); len(got) != 0 {
		t.Fatalf("Parse(empty) = %#v", got)
	}
}
