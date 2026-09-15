// Package rules owns Zipit's exclusion-rule semantics.
package rules

// Rule is the normalized representation of one exclusion pattern.
type Rule struct {
	Pattern string
	Source  string
	Line    int

	directory bool
	basename  bool
	subtree   bool
}
