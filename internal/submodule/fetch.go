package submodule

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/go-kit/log"
)

type repoType int

const (
	gitHub repoType = iota
	gitLab
)

type Info struct {
	Branch        string
	Commit        string // Optional: if specified, use this commit instead of parsing from branch
	SubmodulePath string
	URL           string
}

type Module struct {
	Path   string
	URL    string
	Commit string
}

// Parse the Info into a Module.
func (i Info) Parse() (Module, error) {
	// Use commit if specified, otherwise fall back to branch
	ref := i.Commit
	if ref == "" {
		ref = i.Branch
	}

	// Always parse submodule commits to get the actual submodule commit hash
	infos, err := getSubmoduleCommits(i.URL, ref)
	if err != nil {
		return Module{}, fmt.Errorf("failed to parse submodule commits: %w", err)
	}

	for _, module := range infos {
		if module.Path == i.SubmodulePath {
			return module, nil
		}
	}
	return Module{}, fmt.Errorf("failed to find commit for submodule %q", i.SubmodulePath)
}

func buildRawURL(repoURL, branch, filePath string) (string, int) {
	rt := gitHub
	if strings.Contains(repoURL, "gitlab") {
		rt = gitLab
	}

	switch rt {
	case gitHub:
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", extractRepoPath(repoURL), branch, filePath), int(gitHub)
	case gitLab:
		return fmt.Sprintf("%s/-/raw/%s/%s", repoURL, branch, filePath), int(gitLab)
	default:
		return "", 0
	}
}

func extractRepoPath(repoURL string) string {
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[1:], "/")
	}
	return repoURL
}

func fetchGitModules(repoURL, branch string) (string, int, error) {
	url, rt := buildRawURL(repoURL, branch, ".gitmodules")
	resp, err := http.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch .gitmodules: %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("failed to fetch .gitmodules: %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read .gitmodules: %w", err)
	}

	return string(body), rt, nil
}

func parseGitModules(content string) ([]Module, error) {
	var submodules []Module
	var current Module

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[submodule ") {
			if current.Path != "" {
				submodules = append(submodules, current)
			}
			current = Module{}
		} else if strings.Contains(line, "path = ") {
			current.Path = strings.TrimSpace(strings.Split(line, "=")[1])
		} else if strings.Contains(line, "url = ") {
			current.URL = strings.TrimSpace(strings.Split(line, "=")[1])
		}
	}

	if current.Path != "" {
		submodules = append(submodules, current)
	}

	return submodules, scanner.Err()
}

func fetchSubmoduleCommit(repoType repoType, repoURL, branch, submodulePath string) (string, error) {
	var url string

	switch repoType {
	case gitHub:
		url = fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s", extractRepoPath(repoURL), submodulePath, branch)
		resp, err := http.Get(url)
		if err != nil {
			return "", fmt.Errorf("failed to fetch submodule info: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read submodule info: %w", err)
		}

		commitRegex := regexp.MustCompile(`"sha":\s*"([a-f0-9]{40})"`)
		matches := commitRegex.FindStringSubmatch(string(body))
		if len(matches) > 1 {
			return matches[1], nil
		}

	case gitLab:
		// Use GitLab API to get repository tree
		repoPath := extractRepoPath(repoURL)
		// Extract GitLab base URL from the repository URL
		gitlabBaseURL := strings.Split(repoURL, "/")[0] + "//" + strings.Split(repoURL, "/")[2]
		apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?ref=%s",
			gitlabBaseURL,
			strings.ReplaceAll(repoPath, "/", "%2F"),
			branch)

		resp, err := http.Get(apiURL)
		if err != nil {
			return "", fmt.Errorf("failed to fetch submodule info: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read submodule info: %w", err)
		}

		// Look for submodule entry with type "commit" in tree API response
		submodulePattern := fmt.Sprintf(`"id":"([a-f0-9]{40})"[^}]*"name":"%s"[^}]*"type":"commit"`, regexp.QuoteMeta(submodulePath))
		commitRegex := regexp.MustCompile(submodulePattern)
		matches := commitRegex.FindStringSubmatch(string(body))
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "unknown", nil
}

func getSubmoduleCommits(repoURL, branch string) ([]Module, error) {
	gitmodulesContent, rt, err := fetchGitModules(repoURL, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch .gitmodules: %w", err)
	}

	submodules, err := parseGitModules(gitmodulesContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse .gitmodules: %w", err)
	}

	for i := range submodules {
		commit, err := fetchSubmoduleCommit(repoType(rt), repoURL, branch, submodules[i].Path)
		if err != nil {
			logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
			_ = logger.Log("msg", "Warning: failed to get commit for submodule", "path", submodules[i].Path, "error", err)
			commit = "unknown"
		}
		submodules[i].Commit = commit
	}

	return submodules, nil
}

type gitHubCommit struct {
	SHA string `json:"sha"`
}

func GithubLatestCommit(apiURL string) (string, error) {
	url := fmt.Sprintf("%s/commits/main", apiURL)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var commit gitHubCommit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return "", err
	}

	return commit.SHA, nil
}
