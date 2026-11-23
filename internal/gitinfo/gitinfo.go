package gitinfo

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// RepoInfo captures git metadata for a directory.
type RepoInfo struct {
	IsGitRepo     bool
	RootDir       string
	DefaultBranch string
	RemoteOwner   string
	RemoteRepo    string
}

var (
	sshRemoteRe   = regexp.MustCompile(`^git@[^:]+:([^/]+)/([^/]+?)(?:\.git)?$`)
	httpsRemoteRe = regexp.MustCompile(`^https?://[^/]+/([^/]+)/([^/]+?)(?:\.git)?$`)
)

// Inspect gathers git metadata for dir without failing if git is absent or dir isn't a repo.
func Inspect(dir string) RepoInfo {
	info := RepoInfo{
		DefaultBranch: "main",
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	if root, err := runGit(absDir, "rev-parse", "--show-toplevel"); err == nil && root != "" {
		info.IsGitRepo = true
		info.RootDir = root
	} else {
		return info
	}

	if branch := detectDefaultBranch(absDir); branch != "" {
		info.DefaultBranch = branch
	}

	if remote, err := runGit(absDir, "remote", "get-url", "origin"); err == nil && remote != "" {
		owner, repo := ParseRemoteOwnerRepo(remote)
		info.RemoteOwner = owner
		info.RemoteRepo = repo
	}

	return info
}

// ParseRemoteOwnerRepo extracts owner and repo name from SSH or HTTPS git URLs.
func ParseRemoteOwnerRepo(remoteURL string) (string, string) {
	u := strings.TrimSpace(remoteURL)
	if m := sshRemoteRe.FindStringSubmatch(u); len(m) == 3 {
		return m[1], m[2]
	}
	if m := httpsRemoteRe.FindStringSubmatch(u); len(m) == 3 {
		return m[1], m[2]
	}
	return "", ""
}

func detectDefaultBranch(dir string) string {
	// Try origin/HEAD symbolic ref first.
	if out, err := runGit(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && out != "" {
		parts := strings.SplitN(out, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			return parts[1]
		}
	}

	// Fallback: check current branch if it's main or master.
	if cur, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		if cur == "main" || cur == "master" {
			return cur
		}
	}

	// Check if master branch exists when main does not.
	if _, err := runGit(dir, "show-ref", "--verify", "--quiet", "refs/heads/main"); err == nil {
		return "main"
	}
	if _, err := runGit(dir, "show-ref", "--verify", "--quiet", "refs/heads/master"); err == nil {
		return "master"
	}

	return "main"
}

// DetectGHUser attempts to read the authenticated GitHub username via the `gh` CLI.
// Returns "" if gh is not installed or not authenticated.
func DetectGHUser() string {
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	user := strings.TrimSpace(string(out))
	if strings.ContainsAny(user, " \t\n/") {
		return ""
	}
	return user
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
