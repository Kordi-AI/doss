package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

var githubAPIBase = "https://api.github.com"

func ensureGitHubDeviceKey(dir, token string) error {
	repo, ok := githubRepoForVault(dir)
	if !ok {
		return nil
	}
	id := vault.DeviceID(dir)
	dev, err := vault.DeviceRecord(dir, id)
	if err == nil && dev.DeployKeyID > 0 && dev.GitHubRepo == repo {
		keyPath := devicePrivateKeyPath(dir, id)
		if _, err := os.Stat(keyPath); err != nil {
			return fmt.Errorf("device deploy key is recorded, but local private key is missing at %s; reattach this device with `doss init --from %s`", keyPath, repo)
		}
		return configureGitHubSSH(dir, repo, keyPath)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if token == "" {
		token = githubTokenFromOrigin(dir)
	}
	if token == "" {
		var err error
		token, err = githubAuthToken()
		if err != nil {
			return fmt.Errorf("GitHub vault sync needs a per-device deploy key; authenticate with `gh auth login` and rerun `doss sync`: %w", err)
		}
	}
	keyPath, publicKey, fingerprint, err := ensureDeviceSSHKey(dir, id)
	if err != nil {
		return err
	}
	title := "doss " + id
	keyID, err := githubAddDeployKey(token, repo, title, publicKey)
	if err != nil {
		return err
	}
	if err := configureGitHubSSH(dir, repo, keyPath); err != nil {
		return err
	}
	if _, err := vault.SetDeviceDeployKey(dir, id, repo, title, fingerprint, keyID); err != nil {
		return err
	}
	return nil
}

func revokeDeviceDeployKey(dir, id string) (bool, error) {
	dev, err := vault.DeviceRecord(dir, id)
	if err != nil {
		return false, err
	}
	if dev.DeployKeyID == 0 || dev.GitHubRepo == "" {
		return false, nil
	}
	token, err := githubAuthToken()
	if err != nil {
		return true, fmt.Errorf("could not revoke GitHub deploy key for %s; authenticate with `gh auth login` and retry: %w", id, err)
	}
	if err := githubDeleteDeployKey(token, dev.GitHubRepo, dev.DeployKeyID); err != nil {
		return true, err
	}
	return true, nil
}

func githubRepoForVault(dir string) (string, bool) {
	if out, err := gitx.Run(dir, "config", "--local", "--get", "doss.githubRepo"); err == nil {
		if repo := strings.TrimSpace(out); githubRepoValid(repo) {
			return repo, true
		}
	}
	if out, err := gitx.Run(dir, "remote", "get-url", "origin"); err == nil {
		return githubRepoFromRef(strings.TrimSpace(out))
	}
	return "", false
}

func githubTokenFromOrigin(dir string) string {
	out, err := gitx.Run(dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(out))
	if err != nil || u.Host != "github.com" || u.User == nil {
		return ""
	}
	if password, ok := u.User.Password(); ok && password != "" {
		return password
	}
	return u.User.Username()
}

func configureGitHubSSH(dir, repo, keyPath string) error {
	sshURL := githubSSHURL(repo)
	if gitx.HasRemote(dir) {
		if _, err := gitx.Run(dir, "remote", "set-url", "origin", sshURL); err != nil {
			return err
		}
	} else if _, err := gitx.Run(dir, "remote", "add", "origin", sshURL); err != nil {
		return err
	}
	if _, err := gitx.Run(dir, "config", "--local", "core.sshCommand", sshCommand(keyPath)); err != nil {
		return err
	}
	if _, err := gitx.Run(dir, "config", "--local", "doss.githubRepo", repo); err != nil {
		return err
	}
	return nil
}

func ensureDeviceSSHKey(dir, id string) (privatePath, publicKey, fingerprint string, err error) {
	privatePath = devicePrivateKeyPath(dir, id)
	publicPath := privatePath + ".pub"
	if _, err := os.Stat(privatePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(privatePath), 0o700); err != nil {
			return "", "", "", err
		}
		if _, err := exec.LookPath("ssh-keygen"); err != nil {
			return "", "", "", fmt.Errorf("ssh-keygen is required to create a per-device deploy key")
		}
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", "doss:"+id, "-f", privatePath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", "", "", fmt.Errorf("ssh-keygen failed: %s", strings.TrimSpace(string(out)))
		}
		_ = os.Chmod(privatePath, 0o600)
	} else if err != nil {
		return "", "", "", err
	}
	raw, err := os.ReadFile(publicPath)
	if err != nil {
		return "", "", "", err
	}
	fingerprint = sshFingerprint(publicPath)
	return privatePath, strings.TrimSpace(string(raw)), fingerprint, nil
}

func devicePrivateKeyPath(dir, id string) string {
	return filepath.Join(dir, "local", "keys", id+"_ed25519")
}

func sshCommand(keyPath string) string {
	return "ssh -i " + shellQuote(keyPath) + " -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
}

func sshFingerprint(publicPath string) string {
	out, err := exec.Command("ssh-keygen", "-lf", publicPath, "-E", "sha256").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		return fields[1]
	}
	return ""
}

var githubAuthToken = func() (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("GitHub CLI not found")
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("gh returned an empty token")
	}
	return token, nil
}

func githubCreateRepoWithToken(token, name string) (sshURL, fullName string, err error) {
	req, err := githubRequest("POST", "/user/repos", token,
		map[string]any{"name": name, "private": true, "description": "My private Doss memory vault"})
	if err != nil {
		return "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("GitHub said %s — check auth (repo/admin permission needed) and that %q doesn't already exist", resp.Status, name)
	}
	var out struct {
		FullName string `json:"full_name"`
		SSHURL   string `json:"ssh_url"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.FullName == "" || out.SSHURL == "" {
		return "", "", fmt.Errorf("unexpected GitHub response")
	}
	return out.SSHURL, out.FullName, nil
}

func githubAddDeployKey(token, repo, title, publicKey string) (int64, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return 0, fmt.Errorf("invalid GitHub repo %q", repo)
	}
	req, err := githubRequest("POST", "/repos/"+owner+"/"+name+"/keys", token,
		map[string]any{"title": title, "key": publicKey, "read_only": false})
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("GitHub deploy key create failed for %s: %s", repo, strings.TrimSpace(string(body)))
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ID == 0 {
		return 0, fmt.Errorf("unexpected GitHub deploy key response")
	}
	return out.ID, nil
}

func githubDeleteDeployKey(token, repo string, keyID int64) error {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return fmt.Errorf("invalid GitHub repo %q", repo)
	}
	req, err := githubRequest("DELETE", "/repos/"+owner+"/"+name+"/keys/"+strconv.FormatInt(keyID, 10), token, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return fmt.Errorf("GitHub deploy key delete failed for %s key %d: %s", repo, keyID, strings.TrimSpace(string(body)))
}

func githubRequest(method, apiPath, token string, body any) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, strings.TrimRight(githubAPIBase, "/")+apiPath, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func githubRepoFromRef(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, ".git")
	switch {
	case strings.HasPrefix(ref, "git@github.com:"):
		return cleanGithubRepo(strings.TrimPrefix(ref, "git@github.com:"))
	case strings.HasPrefix(ref, "ssh://git@github.com/"):
		return cleanGithubRepo(strings.TrimPrefix(ref, "ssh://git@github.com/"))
	case strings.Contains(ref, "github.com/"):
		_, after, _ := strings.Cut(ref, "github.com/")
		if at := strings.LastIndex(after, "@"); at >= 0 {
			after = after[at+1:]
		}
		return cleanGithubRepo(after)
	default:
		if strings.Count(ref, "/") == 1 {
			return cleanGithubRepo(ref)
		}
		return "", false
	}
}

func cleanGithubRepo(s string) (string, bool) {
	s = strings.Trim(strings.TrimSpace(s), "/")
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return "", false
	}
	repo := parts[0] + "/" + parts[1]
	return repo, githubRepoValid(repo)
}

func githubRepoValid(repo string) bool {
	if repo == "" {
		return false
	}
	owner, name, ok := strings.Cut(repo, "/")
	return ok && owner != "" && name != "" && !strings.Contains(name, "/")
}

func githubSSHURL(repo string) string {
	return "git@github.com:" + repo + ".git"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
