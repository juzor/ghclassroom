package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Result struct {
	Login string
	Repo  string
	Error error
}

// CloneRepo clones repoURL into targetDir/{repoName}. If the destination
// already exists it runs git pull instead.
func CloneRepo(repoURL, targetDir string) error {
	repoName := repoURL
	if idx := strings.LastIndex(repoName, "/"); idx >= 0 {
		repoName = repoName[idx+1:]
	}
	repoName = strings.TrimSuffix(repoName, ".git")
	destPath := filepath.Join(targetDir, repoName)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := os.Stat(destPath); err == nil {
		out, err := exec.CommandContext(ctx, "git", "-C", destPath, "pull").CombinedOutput()
		if err != nil {
			return fmt.Errorf("git pull: %w\n%s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	out, err := exec.CommandContext(ctx, "git", "clone", repoURL, destPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CloneAll clones all repos using a pool of 3 workers. Each result is sent to
// progress; the channel is closed when all workers finish.
func CloneAll(
	repos []struct{ Login, URL string },
	targetDir string,
	progress chan<- Result,
) error {
	jobs := make(chan struct{ Login, URL string }, len(repos))

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				err := CloneRepo(r.URL, targetDir)
				progress <- Result{Login: r.Login, Repo: r.URL, Error: err}
			}
		}()
	}

	for _, r := range repos {
		jobs <- r
	}
	close(jobs)

	wg.Wait()
	close(progress)
	return nil
}
