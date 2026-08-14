package gopcre2

// Callout support is implemented directly in the VM (vm.go).
// The CalloutFunc and CalloutBlock types are defined in match.go.
//
// Callouts are triggered by (?C), (?Cn), or (?C"text") in the pattern.
// During matching, when the VM reaches an OpCallout instruction, it invokes
// the registered CalloutFunc (if any). If the callback returns non-zero,
// the VM backtracks from that point.
//
// Example usage:
//
//	re := gopcre2.MustCompile(`foo(?C1)bar`)
//	re.SetCallout(func(cb *gopcre2.CalloutBlock) int {
//	    fmt.Printf("Callout %d at position %d\n", cb.CalloutNumber, cb.CurrentPosition)
//	    return 0 // continue matching
//	})
//	re.MatchString("foobar")
