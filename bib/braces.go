package bib

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// BraceNode represents a single node in a parse tree of a brace string If
// len(Children) == 0, then Leaf holds the leaf text for this node Each node in
// the tree is either a root, a internal node, or a leaf (duh), with the
// following meanings:
//
//      - only leaf nodes contain text
//      - internal nodes represent {}-deliminated strings
//      - root nodes represent the entire string and DO NOT correspond to {}-diminated strings.
//
// For example, "{foo moo man}" leads to the tree with a ROOT node, which has a
// single INTERNAL node child, which itself has a single LEAF node.

type BraceNode struct {
	Children []*BraceNode
	Leaf     string
}

// ParseBraceTree converts a string into a tree of BraceNodes
func ParseBraceTree(s string) (*BraceNode, int) {

	// walk thru runes, if

	me := &BraceNode{
		Children: make([]*BraceNode, 0),
	}
	accum := ""
	saveAccum := func() {
		if len(accum) > 0 {
			me.Children = append(me.Children, &BraceNode{Leaf: accum})
			accum = ""
		}
	}
	i := 0
	for i < len(s) {
		r, iskip := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '{':
			saveAccum()
			c, nread := ParseBraceTree(s[i+iskip:])
			me.Children = append(me.Children, c)
			iskip += nread
		case '}':
			saveAccum()
			return me, i + iskip
		default:
			accum += string(r)
		}
		i += iskip
	}
	saveAccum()
	return me, i
}

// printIndent prints a given number of spaces (for debugging)
func printIndent(indent int) {
	for indent > 0 {
		fmt.Print(" ")
		indent--
	}
}

// IsLeaf() returns true if this BraceNode represents a leaf
func (bn *BraceNode) IsLeaf() bool {
	return len(bn.Children) == 0
}

// IsEntireStringBraced() returns true iff the entire string is enclosed in a
// {} {Moo}{Bar} returns false, but {Moo Bar} returns true. Technically, this
// is checked by testing whether the root node has a single child that is
// itself a braceNode (and not a leaf)
func (bn *BraceNode) IsEntireStringBraced() bool {
	return len(bn.Children) == 1 && !bn.Children[0].IsLeaf()
}

// Flatten() return the string represented by the tree as a string
func (bn *BraceNode) Flatten() string {
	return bn.flatten(true)
}

// flatten is a helper function that does the work of Flatten() [it exists
// to handle root nodes specially]
func (bn *BraceNode) flatten(isroot bool) string {
	if bn.IsLeaf() {
		return bn.Leaf
	} else {
		words := make([]string, 0)
		for _, c := range bn.Children {
			words = append(words, c.flatten(false))
		}
		if isroot {
			return strings.Join(words, "")
		} else {
			return "{" + strings.Join(words, "") + "}"
		}
	}
}

//PrintBraceTree is used for debugging --- it prints the brace tree to stdout
//in a simple format.
func PrintBraceTree(b *BraceNode, indent int) {
	printIndent(indent)
	if b.Leaf != "" {
		fmt.Printf("LEAF \"%s\"\n", b.Leaf)
	} else {
		fmt.Println("NODE")
		for _, c := range b.Children {
			PrintBraceTree(c, indent+2)
		}
	}
}

// SplitWords returns an array of strings, where each entry is either a
// sequence of non-whitespace chars, or a sequence of whitepace chars. s ==
// strings.Join(return, "")
func SplitWords(s string) []string {
	words := make([]string, 0)
	word := ""
	c, _ := utf8.DecodeRuneInString(s)
	inspace := unicode.IsSpace(c)
	for _, r := range s {
		if inspace && unicode.IsSpace(r) {
			word += string(r)
		} else if !inspace && !unicode.IsSpace(r) {
			word += string(r)
		} else {
			inspace = unicode.IsSpace(r)
			if word != "" {
				words = append(words, word)
			}
			word = string(r)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}

// ContainsNoBraces returns true if the tree contains no {}-deliminated substrings
func (bn *BraceNode) ContainsNoBraces() bool {
	return len(bn.Children) == 1 && bn.Children[0].IsLeaf()
}

// FlattenToMinBraces tries to smartly {}-deliminate the smallest regions in
// the text that correspond to things that need {}-delimination: strange-case
// (mRNA) and quotes (").  This will *only* change strings if it looks like the
// user didn't put any thought into it: specifically, only if the entire string
// is {] or none of the string is {}.
func (bn *BraceNode) FlattenToMinBraces() string {
	// only modify if it looks like the user didn't really think about
	// the braces (the most common case)
	var children []*BraceNode = nil
	if bn.IsEntireStringBraced() {
		children = bn.Children[0].Children
	} else if bn.ContainsNoBraces() {
		children = bn.Children
	}

	if children != nil {
		words := make([]string, 0)

		for _, c := range children {
			// for leaf children, we iterate through the words
			if c.IsLeaf() {
				for _, w := range SplitWords(c.Leaf) {
					if IsStrangeCase(w) || HasQuote(w) {
						words = append(words, "{"+w+"}")
					} else {
						words = append(words, w)
					}
				}
				// for non-leaf children, we just flatten as normal
			} else {
				words = append(words, c.flatten(false))
			}

		}
		return strings.Join(words, "")

	} else {
		return bn.Flatten()
	}
}

// split s on occurances of sep that are not contained in a { } block. If whitespace is
// true, the it requires that the string be surrounded by whitespace (or the string
// boundaries)
func splitOnTopLevelString(s, sep string, whitespace bool) []string {
	nbraces := 0
	lastend := 0
	split := make([]string, 0)

	following := ' '
	prevchar := ' '

	for i, r := range s {
		switch r {
		case '{':
			nbraces++
		case '}':
			nbraces--
		}
		// if we're not in a nested brace and we have a match
		if nbraces == 0 && i >= lastend && i < len(s)-len(sep)+1 && strings.ToLower(s[i:i+len(sep)]) == sep {
			// get the run following the end of the match
			following = ' '
			if i+len(sep) < len(s)-1 {
				following, _ = utf8.DecodeRuneInString(s[i+len(sep):])
			}

			// either we dont' care about whitespace or
			// the match is surrounded by whitespace on each side
			if !whitespace || (unicode.IsSpace(prevchar) && unicode.IsSpace(following)) {
				// then save the split
				split = append(split, s[lastend:i])
				lastend = i + len(sep)
			}

		}
		prevchar = r
	}
	last := s[lastend:]
	split = append(split, last)

	return split
}

// splitOnTopLevel splits s on whitespace separated words, but treats {}-deliminated substrings
// as a unit
func splitOnTopLevel(s string) []string {
	nbraces := 0
	word := ""
	words := make([]string, 0)
	s = strings.TrimSpace(s)
	for _, r := range s {
		switch r {
		case '{':
			nbraces++
		case '}':
			nbraces--
		}
		// if we are at a top-level space
		if nbraces == 0 && unicode.IsSpace(r) {
			// and there is a current word, save it
			if len(word) > 0 {
				words = append(words, word)
				word = ""
			}
			// if we are in {} or non-space, add to current word
		} else {
			word += string(r)
		}
	}
	if len(word) > 0 {
		words = append(words, word)
	}
	return words
}

// IsStrangeCase returns true iff s has a capital letter someplace other than
// the first position and not preceeded by a - (so Whole-Genome is not in
// strange case. We also ignore punctuation at the start, so "(Whole-Genome" is
// also not in strange case. mRNA is.
func IsStrangeCase(s string) bool {
	p := 0
	prevRune := ' '
	for _, r := range s {
		if p > 0 && prevRune != '-' && unicode.IsUpper(r) {
			return true
		}
		prevRune = r
		// we ignore punct at the start of the word to handle cases like "(This"
		if !unicode.IsPunct(r) {
			p++
		}
	}
	return false
}

// HasQuote returns true iff the string has a " someplace in it.
func HasQuote(w string) bool {
	for _, r := range w {
		switch r {
		case '"':
			return true
		}
	}
	return false
}