package main

import "strings"

func normalizeFolderSegment(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.Trim(name, " /")
	if name == "" {
		return ""
	}

	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '/'
	})
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		part = strings.Join(strings.Fields(part), " ")
		if part != "" {
			clean = append(clean, part)
		}
	}

	if len(clean) == 0 {
		return ""
	}
	return strings.Join(clean, "-")
}

func normalizeFolderPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.Trim(path, " /")
	if path == "" {
		return ""
	}

	parts := strings.Split(path, "/")
	stack := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		segment := normalizeFolderSegment(part)
		if segment == "" {
			continue
		}
		stack = append(stack, segment)
	}

	return strings.Join(stack, "/")
}

func splitNormalizedFolderPath(path string) []string {
	path = normalizeFolderPath(path)
	if path == "" {
		return []string{}
	}
	parts := strings.Split(path, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return clean
}

