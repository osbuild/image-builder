// Package pathpolicy implements a system for checking mountpoint paths
//
// A list of policies for path prefixes and their subdirectories is created by
// passing a map to NewPathPolicies that describes whether a prefix should be
// denied (Deny: true), or if the prefix is allowed, but subdirectories
// rejected (Exact: true).  If Deny and Exact are both false then the prefix
// and all subdirectories are allowed.
//
// The check function finds the longest matching prefix in the map and uses the
// policy that has been defined for that prefix to determine if the requested
// path is allowed.
//
// Example:
//
//	policies := NewPathPolicies(map[string]PathPolicy{
//	     "/":          {},
//	     "/boot":      {Deny: true},
//	     "/boot/efi":  {Exact: true},
//	     "/usr":       {},
//	     "/usr/share": {Deny: true},
//	})
//
// This will allow a mountpoint at /boot/efi but not allow /boot or any other
// paths under /boot.
// It will deny a mountpoint at /var/home but allow /var and all other paths
// under /var.
//
// A disadvantage of this is that you cannot express a policy that would allow
// arbitrary paths under /var, but deny /var/home and /var itself. Since it
// finds the longest matching prefix it would find /var with Deny: true and
// deny a path like /var/foo
package pathpolicy

import (
	"fmt"
	"path"
)

type PathPolicy struct {
	Deny  bool // explicitly do not allow this entry
	Exact bool // require and exact match, no subdirs
}

type PathPolicies struct {
	pathTrie *pathTrie[PathPolicy]
}

// Create a new PathPolicies trie from a map of path to PathPolicy
func NewPathPolicies(entries map[string]PathPolicy) *PathPolicies {
	return &PathPolicies{
		pathTrie: newPathTrieFromMap(entries),
	}
}

// Check a given path against the PathPolicies
func (pol *PathPolicies) Check(fsPath string) error {
	// Quickly check we have a path and it is absolute
	if fsPath == "" || fsPath[0] != '/' {
		return fmt.Errorf("path %q must be absolute", fsPath)
	}

	// ensure that only clean paths are valid
	if fsPath != path.Clean(fsPath) {
		return fmt.Errorf("path %q must be canonical", fsPath)
	}

	node, left := pol.pathTrie.Lookup(fsPath)
	policy := node.Payload

	// 1) path is explicitly not allowed or
	// 2) a subpath was match but an explicit match is required
	if policy.Deny || (policy.Exact && len(left) > 0) {
		return fmt.Errorf("path %q is not allowed", fsPath)
	}

	// exact match or recursive path allowed
	return nil
}
