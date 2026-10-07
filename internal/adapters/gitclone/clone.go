// Package gitclone clones and updates git remotes into a DockVista-owned
// directory using go-git, so the distroless image never needs a git binary.
package gitclone

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"dockvista/internal/core/domain"
)

const cloneTimeout = 45 * time.Second

type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) Clone(ctx context.Context, dir, remote, ref, username, token string) error {
	ctx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrGitClone, err)
	}
	try := func(refName plumbing.ReferenceName) error {
		_ = os.RemoveAll(dir)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		opts := &git.CloneOptions{
			URL:           remote,
			Auth:          auth(username, token),
			Depth:         1,
			SingleBranch:  true,
			ReferenceName: refName,
		}
		_, err := git.PlainCloneContext(ctx, dir, false, opts)
		return err
	}
	var err error
	if ref != "" && ref != "HEAD" {
		err = try(plumbing.NewBranchReferenceName(ref))
		if err == nil {
			return nil
		}
		if tagErr := try(plumbing.NewTagReferenceName(ref)); tagErr == nil {
			return nil
		}
	} else {
		err = try("")
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("%w: %v", domain.ErrGitClone, err)
}

func (c *Client) Update(ctx context.Context, dir, remote, ref, username, token string) error {
	ctx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return c.Clone(ctx, dir, remote, ref, username, token)
	}
	a := auth(username, token)
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Auth:       a,
		Force:      true,
		Depth:      1,
		Tags:       git.AllTags,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return fmt.Errorf("%w: %v", domain.ErrGitClone, err)
	}
	hash, err := resolveRef(repo, ref)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrGitClone, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrGitClone, err)
	}
	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: hash}); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrGitClone, err)
	}
	return nil
}

func resolveRef(repo *git.Repository, ref string) (plumbing.Hash, error) {
	if ref == "" {
		ref = domain.DefaultGitRef
	}
	for _, name := range []plumbing.ReferenceName{
		plumbing.NewRemoteReferenceName("origin", ref),
		plumbing.NewBranchReferenceName(ref),
		plumbing.NewTagReferenceName(ref),
	} {
		if r, err := repo.Reference(name, true); err == nil {
			return r.Hash(), nil
		}
	}
	h, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return *h, nil
}

func auth(username, token string) transport.AuthMethod {
	if token == "" && username == "" {
		return nil
	}
	if username == "" {
		username = "git"
	}
	return &http.BasicAuth{Username: username, Password: token}
}
