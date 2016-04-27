package unfurlist

import "sort"

// prefixMap allows fast checks against predefined set of prefixes.
// Uninitialized/empty prefixMap considers any string as matching.
type prefixMap struct {
	prefixes map[string]struct{}
	lengths  []int // sorted, smaller first
}

// newPrefixMap initializes new prefixMap from given slice of prefixes
func newPrefixMap(prefixes []string) *prefixMap {
	if len(prefixes) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(prefixes))
	l1 := make([]int, 0, len(prefixes))
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		m[p] = struct{}{}
		l1 = append(l1, len(p))
	}
	if len(l1) == 0 {
		return nil
	}
	sort.Ints(l1)
	// remove duplicates
	l2 := l1[:1]
	for i := 1; i < len(l1); i++ {
		if l1[i] != l1[i-1] {
			l2 = append(l2, l1[i])
		}
	}
	return &prefixMap{
		prefixes: m,
		lengths:  l2,
	}
}

// Match validates string against set of prefixes. It returns true only if
// prefixMap is non-empty and string matches at least one prefix.
func (m *prefixMap) Match(url string) bool {
	if m == nil || m.prefixes == nil {
		return false
	}
	sLen := len(url)
	if sLen < m.lengths[0] {
		return false
	}
	for _, x := range m.lengths {
		if sLen < x {
			continue
		}
		if _, ok := m.prefixes[url[:x]]; ok {
			return true
		}
	}
	return false
}
