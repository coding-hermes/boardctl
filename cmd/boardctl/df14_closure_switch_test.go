package main

// DF-BOARDCTL-14: the usage-closure meta-test must DERIVE its expectations
// from the command switch in main.go's run(), not pin a count.
//
// History: DF-BOARDCTL-7 pinned `want := 16` in
// TestEveryUsageClosureEndsWithRealNewline. Commit 77f00c6 made the root
// --help list install (16 commands) while the test still pinned 16 — so a
// 17th command added to the switch WITHOUT a usage closure would still pass
// the pinned count. This file replaces the pinned count with a derived
// expectation:
//
//	derived command set = every `case "<verb>":` inside run()'s `switch cmd`
//	expected closures   = one per command, counted over the same files the
//	                      DF-BOARDCTL-7 walker scans (productionGoFiles)
//
// ATTRIBUTION RULE (documented once here, implemented in attributedClosures):
//
//	A closure is attributable to command V when the most recent preceding
//	line in the same file declares that command's flagset —
//	`newFlagSet("V")` — and the attribution is consumed by the closure (a
//	second closure before a fresh newFlagSet declaration is reported as
//	unattributed rather than double-counted). Why this rule and not line
//	order vs the switch: closures carry NO command name in source, and
//	main.go's switch order need not match file layout (import.go,
//	render.go, serve.go, install.go each carry one closure for their
//	command in the same package). Every command function in this package
//	declares its flagset immediately before arming fs.Usage, so "latest
//	newFlagSet wins" survives closures moving anywhere within their own
//	command function or across same-package files.
//
// VERB-DERIVATION RULE (switchVerbsFromLines): a `case "X":` inside the
// switch contributes verb X when X is bare-verb shaped (lowercase
// letter/digit/hyphen). Cases whose body prints usageText DIRECTLY are
// excluded by that shape — today only `case "help", "-h", "--help":`, which
// prints the top-level usage and owns no fs.Usage closure; a real command
// dispatches into a cmd* function and never references usageText in its
// case body. Excluding by shape (not a hardcoded name list) keeps the
// parser honest when the help case grows more flag spellings.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// switchVerbs parses the real main.go and returns the command verbs from
// run()'s switch, in appearance order.
func switchVerbs(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("cannot read main.go: %v", err)
	}
	verbs, perr := switchVerbsFromLines(strings.Split(string(body), "\n"))
	if perr != nil {
		t.Fatalf("main.go: %v", perr)
	}
	return verbs
}

// switchVerbsFromLines extracts the command verbs from a main.go-shaped
// body: locate `switch cmd {`, find its closing brace by depth counting,
// then take every `case "<verb>":` whose case body does not print usageText
// directly (the help case). Pure function over []string so the negative
// control can feed synthetic lines without touching the repo tree.
func switchVerbsFromLines(lines []string) ([]string, error) {
	switchIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "switch cmd {") {
			if switchIdx >= 0 {
				return nil, fmt.Errorf("more than one 'switch cmd {' (lines %d and %d) — re-anchor the meta-test", switchIdx+1, i+1)
			}
			switchIdx = i
		}
	}
	if switchIdx < 0 {
		return nil, fmt.Errorf("no 'switch cmd {' found — run()'s command dispatch moved or was renamed; re-anchor the meta-test")
	}
	end := -1
	depth := 0
	opened := false
	for j := switchIdx; j < len(lines); j++ {
		depth += strings.Count(lines[j], "{")
		if strings.Contains(lines[j], "{") {
			opened = true
		}
		depth -= strings.Count(lines[j], "}")
		if opened && depth <= 0 {
			end = j
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("cannot find the end of the command switch starting at line %d", switchIdx+1)
	}

	caseRe := regexp.MustCompile(`case\s+"([^"]+)"`)
	allQuoted := regexp.MustCompile(`"([^"]+)"`)
	verbRe := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	var verbs []string
	for i := switchIdx; i <= end; i++ {
		m := caseRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		// Case body: this line up to the next case/default or the switch's
		// closing brace.
		bodyEnd := end
		for j := i + 1; j <= end; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if strings.HasPrefix(trimmed, "case ") || strings.HasPrefix(trimmed, "default:") {
				bodyEnd = j - 1
				break
			}
		}
		body := strings.Join(lines[i+1:bodyEnd+1], "\n")
		if strings.Contains(body, "usageText") {
			continue // prints top-level usage itself (help); owns no closure
		}
		for _, tok := range allQuoted.FindAllStringSubmatch(lines[i], -1) {
			if verbRe.MatchString(tok[1]) {
				verbs = append(verbs, tok[1])
			}
		}
	}
	if len(verbs) == 0 {
		return nil, fmt.Errorf("switch starting at line %d yielded zero command cases — the parser broke", switchIdx+1)
	}
	return verbs, nil
}

// flagSetCallRe matches `newFlagSet("<verb>")` — the declaration each
// command function makes before arming fs.Usage.
var flagSetCallRe = regexp.MustCompile(`newFlagSet\("([^"]+)"\)`)

// usageClosureRe matches the closure marker the DF-BOARDCTL-7 walker keys on.
var usageClosureRe = regexp.MustCompile(`fs\.Usage = func\(\)`)

// attributedClosures walks lines and attributes every fs.Usage closure to
// the most recent newFlagSet("<verb>") declaration (see the attribution
// rule in the file comment). Returns verb -> closure count plus the number
// of closures that found no flagset declaration (unattributed).
func attributedClosures(lines []string) (map[string]int, int) {
	attributed := map[string]int{}
	unattributed := 0
	lastVerb := ""
	for _, line := range lines {
		if m := flagSetCallRe.FindStringSubmatch(line); m != nil {
			lastVerb = m[1]
		}
		if usageClosureRe.MatchString(line) {
			if lastVerb == "" {
				unattributed++
			} else {
				attributed[lastVerb]++
				// Consume the attribution: a second closure before a
				// fresh newFlagSet declaration is unattributed, not
				// double-counted against the same verb.
				lastVerb = ""
			}
		}
	}
	return attributed, unattributed
}

// missingClosureMsg is the gate's missing-closure failure. It is a named
// helper so the negative control can pin that the message carries the verb.
func missingClosureMsg(verb string) string {
	return fmt.Sprintf("command %q is in run()'s switch but owns NO fs.Usage closure — a new command shipped without usage text (or its closure lost its newFlagSet(%q) attribution)", verb, verb)
}

// closureGateFailures is the derived-expectation gate as a pure function:
// given the switch verbs and the package's closure attributions it returns
// one message per violation (empty slice = pass):
//   - a switch verb with zero closures (the DF-BOARDCTL-14 defect shape);
//   - a switch verb with more than one closure;
//   - a closure attributed to a verb that is NOT in the switch (orphaned
//     closure — command removed from the switch, closure left behind);
//   - any unattributable closure.
//
// The count is never pinned: a 17th command WITH a closure passes without
// touching this test; a 17th command WITHOUT a closure fails naming it.
func closureGateFailures(verbs []string, attributed map[string]int, unattributed int) []string {
	verbSet := map[string]bool{}
	for _, v := range verbs {
		verbSet[v] = true
	}
	var fails []string
	for _, v := range verbs {
		switch attributed[v] {
		case 0:
			fails = append(fails, missingClosureMsg(v))
		case 1:
			// exactly one closure: healthy
		default:
			fails = append(fails, fmt.Sprintf("command %q owns %d fs.Usage closures, want exactly 1", v, attributed[v]))
		}
	}
	for verb := range attributed {
		if !verbSet[verb] {
			fails = append(fails, fmt.Sprintf("fs.Usage closure attributed to %q, which is NOT a run() switch case — orphaned closure (command removed from the switch without removing its usage closure)", verb))
		}
	}
	if unattributed > 0 {
		fails = append(fails, fmt.Sprintf("%d fs.Usage closure(s) have no preceding newFlagSet(\"<verb>\") — unattributable closure", unattributed))
	}
	return fails
}

// TestUsageClosureGateDerivesFromCommandSwitch is the DF-BOARDCTL-14
// replacement for the pinned `want := 16` fatal: it derives the expected
// command set from main.go's run() switch, attributes every fs.Usage
// closure in the package to a command, and fails naming any command
// missing a closure and any closure missing a command. Per-closure SHAPE
// checks (prints something, real trailing newline) stay in
// TestEveryUsageClosureEndsWithRealNewline below.
func TestUsageClosureGateDerivesFromCommandSwitch(t *testing.T) {
	verbs := switchVerbs(t)

	attributed := map[string]int{}
	unattributed := 0
	for _, f := range productionGoFiles(t) {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		perFile, orphan := attributedClosures(strings.Split(string(body), "\n"))
		unattributed += orphan
		for verb, n := range perFile {
			attributed[verb] += n
		}
	}

	total := 0
	for _, n := range attributed {
		total += n
	}
	if total == 0 {
		t.Fatal("zero fs.Usage closures found across all production files — the walker or file listing broke; the gate is vacuous")
	}
	if unattributed == total {
		t.Fatal("every closure is unattributed — the attribution walker broke; the gate is vacuous")
	}

	for _, fail := range closureGateFailures(verbs, attributed, unattributed) {
		t.Error(fail)
	}
}

// TestUsageClosureGateNegativeControl proves the gate bites: a synthetic
// command in the switch WITHOUT a closure must fail with a message naming
// it. Pure-function test — the scanners take []string lines and nothing is
// written into the repo tree. The control drives the SAME primitives the
// real gate uses (switchVerbsFromLines, attributedClosures,
// closureGateFailures), so it cannot pass while the real gate is broken.
func TestUsageClosureGateNegativeControl(t *testing.T) {
	healthy := []string{
		"func run(args []string) int {",
		"\tswitch cmd {",
		"\tcase \"init\":",
		"\t\terr = cmdInit(boardDir, rest)",
		"\tcase \"newcmd\":",
		"\t\terr = cmdNewcmd(boardDir, rest)",
		"	case \"help\", \"-h\", \"--help\":",
		"		fmt.Fprint(os.Stdout, usageText)",
		"		return 0",
		"	}",
		"}",
		"",
		"func cmdInit(dir string, args []string) error {",
		"	fs := newFlagSet(\"init\")",
		"	fs.Usage = func() { fmt.Fprintf(fs.Output(), \"boardctl init [-C dir]\\n\") }",
		"	_ = dir",
		"	_ = args",
		"	return nil",
		"}",
		"",
		"func cmdNewcmd(dir string, args []string) error {",
		"\tfs := newFlagSet(\"newcmd\")",
		"\tfs.Usage = func() { fmt.Fprintf(fs.Output(), \"boardctl newcmd [-C dir]\\n\") }",
		"\t_ = dir",
		"\t_ = args",
		"\treturn nil",
		"}",
	}

	// Healthy tree: verbs parse (help excluded by shape), attribution is
	// exact, the gate passes.
	verbs, err := switchVerbsFromLines(healthy)
	if err != nil {
		t.Fatalf("synthetic switch parse failed: %v", err)
	}
	if len(verbs) != 2 || verbs[0] != "init" || verbs[1] != "newcmd" {
		t.Fatalf("synthetic switch verbs = %v, want [init newcmd] (help/-h/--help must be excluded)", verbs)
	}
	attr, unattr := attributedClosures(healthy)
	if unattr != 0 || attr["newcmd"] != 1 || attr["init"] != 1 {
		t.Fatalf("healthy synthetic tree misattributed: %v (unattributed=%d)", attr, unattr)
	}
	if fails := closureGateFailures(verbs, attr, unattr); len(fails) != 0 {
		t.Fatalf("healthy synthetic tree must pass the gate, got: %v", fails)
	}

	// Broken tree: newcmd keeps its newFlagSet declaration but the
	// fs.Usage closure is gone. The gate MUST fail naming newcmd.
	var broken []string
	for _, l := range healthy {
		if strings.Contains(l, "fs.Usage = func()") {
			continue
		}
		broken = append(broken, l)
	}
	bVerbs, err := switchVerbsFromLines(broken)
	if err != nil {
		t.Fatalf("broken-tree switch parse failed: %v", err)
	}
	bAttr, bUnattr := attributedClosures(broken)
	if bUnattr != 0 {
		t.Fatalf("broken tree should have zero unattributed closures, got %d", bUnattr)
	}
	if bAttr["newcmd"] != 0 {
		t.Fatalf("broken tree should attribute zero closures to newcmd, got %d", bAttr["newcmd"])
	}
	fails := closureGateFailures(bVerbs, bAttr, bUnattr)
	if len(fails) == 0 {
		t.Fatal("gate did NOT bite on a closure-less synthetic command — it is vacuous")
	}
	named := false
	for _, f := range fails {
		if strings.Contains(f, "newcmd") {
			named = true
		}
	}
	if !named {
		t.Fatalf("gate failures do not name newcmd: %v", fails)
	}
	if !strings.Contains(fails[0], "NO fs.Usage closure") {
		t.Fatalf("unexpected failure class for the missing-closure case: %q", fails[0])
	}

	// Orphan-closure control: a closure with no newFlagSet declaration
	// before it must be reported unattributed — otherwise a closure could
	// drift out of its command function invisibly.
	oAttr, oUnattr := attributedClosures([]string{"\tfs.Usage = func() { }"})
	if oUnattr != 1 || len(oAttr) != 0 {
		t.Fatalf("orphan closure misclassified: %v (unattributed=%d)", oAttr, oUnattr)
	}
	if fails := closureGateFailures([]string{"init"}, map[string]int{}, 1); len(fails) == 0 {
		t.Fatal("gate did not report the unattributed closure")
	}

	// The missing-closure message must carry the verb (pinned via the same
	// helper the gate calls, so editing the message to drop the verb
	// breaks this control).
	if msg := missingClosureMsg("newcmd"); !strings.Contains(msg, "newcmd") {
		t.Fatalf("missing-closure message lost the verb: %q", msg)
	}
}
