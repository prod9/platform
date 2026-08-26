package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"

	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/git"
)

var (
	// CheckoutCacheConfig overrides the cache root used by Checkout.
	CheckoutCacheConfig = fxconfig.Str("CHECKOUT_CACHE")

	userCacheDir = os.UserCacheDir
)

// Source identifies one immutable revision to materialize from a Git remote.
// Username and Password are used only for HTTP(S) fetches and are never persisted.
type Source struct {
	URL      string
	Revision string
	Username string
	Password string
}

// Worktree is a checked-out revision whose resources remain owned until Close.
type Worktree struct {
	Dir string
	SHA string

	mirror string
}

// Checkout materializes source as an independent worktree backed by a shared mirror.
func Checkout(ctx context.Context, source Source) (*Worktree, error) {
	cacheDir, err := checkoutRoot(ctx)
	if err != nil {
		return nil, err
	}
	return checkoutAt(ctx, cacheDir, source)
}

func checkoutRoot(ctx context.Context) (string, error) {
	if configured := fxconfig.Get(cfgFrom(ctx), CheckoutCacheConfig); configured != "" {
		return configured, nil
	}
	root, err := userCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "platform"), nil
}

func checkoutAt(ctx context.Context, cacheDir string, source Source) (*Worktree, error) {
	if source.URL == "" {
		return nil, errors.New("engine: checkout URL is required")
	}
	if source.Revision == "" {
		return nil, errors.New("engine: checkout revision is required")
	}
	if source.Password != "" && source.Username == "" {
		return nil, errors.New("engine: checkout username is required with a password")
	}

	mirror := checkoutMirror(cacheDir, source.URL)
	if err := syncCheckoutMirror(ctx, mirror, source); err != nil {
		return nil, err
	}

	sha, err := git.Run(ctx, mirror, "rev-parse", source.Revision+"^{commit}")
	if err != nil {
		return nil, err
	}
	dir, err := uniqueWorktreeDir(filepath.Join(cacheDir, "work"))
	if err != nil {
		return nil, err
	}
	if _, err := git.Run(ctx, mirror, "worktree", "add", "--detach", dir, sha); err != nil {
		return nil, err
	}

	return &Worktree{Dir: dir, SHA: sha, mirror: mirror}, nil
}

// Close removes the checkout and prunes its mirror's worktree records.
func (w *Worktree) Close(ctx context.Context) error {
	if _, err := git.Run(ctx, w.mirror, "worktree", "remove", "--force", w.Dir); err != nil {
		return err
	}
	_, err := git.Run(ctx, w.mirror, "worktree", "prune")
	return err
}

func syncCheckoutMirror(ctx context.Context, mirror string, source Source) (err error) {
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
		return err
	}
	lock, err := checkoutLock(mirror + ".lock")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Close()) }()

	if _, err := os.Stat(mirror); os.IsNotExist(err) {
		if _, err := git.Run(ctx, filepath.Dir(mirror), "init", "--bare", "-q", mirror); err != nil {
			return err
		}
		if _, err := git.Run(ctx, mirror, "remote", "add", "--mirror=fetch", "origin", source.URL); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	fetchURL, err := checkoutFetchURL(source)
	if err != nil {
		return err
	}
	_, err = git.Run(ctx, mirror, "fetch", "--prune", fetchURL, "+refs/*:refs/*")
	return err
}

func checkoutMirror(cacheDir, sourceURL string) string {
	digest := sha256.Sum256([]byte(sourceURL))
	key := hex.EncodeToString(digest[:])
	return filepath.Join(cacheDir, "git", key+".git")
}

func uniqueWorktreeDir(root string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, "checkout-")
	if err != nil {
		return "", err
	}
	if err := os.Remove(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func checkoutFetchURL(source Source) (string, error) {
	parsed, err := url.Parse(source.URL)
	if err != nil {
		return "", err
	}
	if parsed.User != nil {
		return "", errors.New("engine: checkout source URL must be credential-free")
	}
	if source.Username == "" {
		return source.URL, nil
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return source.URL, nil
	}
	parsed.User = url.UserPassword(source.Username, source.Password)
	return parsed.String(), nil
}

func checkoutLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(file); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func lockFile(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("engine: lock %s: %w", file.Name(), err)
	}
	return nil
}
