package main

// Find a string position in a slice of strings
func pos(element string, elements []string) int {
	for p, v := range elements {
		if v == element {
			return p
		}
	}
	return -1
}

// Check if a string is in a slice of strings
func exists(element string, elements []string) bool {
	for _, v := range elements {
		if v == element {
			return true
		}
	}
	return false
}
