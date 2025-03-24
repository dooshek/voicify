package main

import (
	"fmt"
	"strings"
)

func normalizeRepoURL(repoURL string) string {
	// Add https:// prefix if no protocol is specified
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "http://") &&
		!strings.HasPrefix(repoURL, "git@") && !strings.HasPrefix(repoURL, "ssh://") {
		repoURL = "https://" + repoURL
	}

	// Add .git suffix if not present
	if !strings.HasSuffix(repoURL, ".git") {
		repoURL = repoURL + ".git"
	}

	return repoURL
}

func main() {
	testURLs := []string{
		"github.com/user/repo",
		"github.com/user/repo.git",
		"https://github.com/user/repo",
		"https://github.com/user/repo.git",
		"git@github.com:user/repo",
		"git@github.com:user/repo.git",
	}

	for _, url := range testURLs {
		normalized := normalizeRepoURL(url)
		fmt.Printf("Original: %-35s -> Normalized: %s\n", url, normalized)
	}
}
