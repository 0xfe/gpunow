package cli

import "testing"

func TestParseScpArgsBasic(t *testing.T) {
	flags, src, dst, err := parseScpArgs([]string{"./a.txt", "cluster/0:/tmp/"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(flags) != 0 || src != "./a.txt" || dst != "cluster/0:/tmp/" {
		t.Fatalf("unexpected result: flags=%v src=%s dst=%s", flags, src, dst)
	}
}

func TestParseScpArgsFlags(t *testing.T) {
	flags, src, dst, err := parseScpArgs([]string{"-r", "-P", "2222", "./a", "cluster/0:/tmp/"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(flags); got != 3 {
		t.Fatalf("unexpected flags: %v", flags)
	}
	if src != "./a" || dst != "cluster/0:/tmp/" {
		t.Fatalf("unexpected src/dst: %s %s", src, dst)
	}
}

func TestParseScpArgsInlineValue(t *testing.T) {
	flags, src, dst, err := parseScpArgs([]string{"-P2222", "./a", "cluster/0:/tmp/"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(flags) != 1 || flags[0] != "-P2222" {
		t.Fatalf("unexpected flags: %v", flags)
	}
	if src != "./a" || dst != "cluster/0:/tmp/" {
		t.Fatalf("unexpected src/dst: %s %s", src, dst)
	}
}

func TestParseScpArgsWithSeparator(t *testing.T) {
	flags, src, dst, err := parseScpArgs([]string{"-r", "--", "-weird", "cluster/0:/tmp/"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(flags) != 1 || flags[0] != "-r" {
		t.Fatalf("unexpected flags: %v", flags)
	}
	if src != "-weird" || dst != "cluster/0:/tmp/" {
		t.Fatalf("unexpected src/dst: %s %s", src, dst)
	}
}
