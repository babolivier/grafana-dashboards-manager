package git

import (
	"io/ioutil"
	"os"
	"strings"

	"config"

	"golang.org/x/crypto/ssh"
	gogit "gopkg.in/src-d/go-git.v4"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

func Sync(cfg config.GitSettings) (r *gogit.Repository, err error) {
	auth, err := getAuth(cfg.User, cfg.PrivateKeyPath)
	if err != nil {
		return
	}

	exists, err := dirExists(cfg.ClonePath)
	if err != nil {
		return
	}

	if exists {
		r, err = pull(cfg.ClonePath, auth)
	} else {
		r, err = clone(cfg.URL, cfg.ClonePath, auth)
	}

	return
}

func getAuth(user string, privateKeyPath string) (*gitssh.PublicKeys, error) {
	privateKey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return &gitssh.PublicKeys{User: "git", Signer: signer}, nil
}

func clone(repo string, clonePath string, auth *gitssh.PublicKeys) (*gogit.Repository, error) {
	return gogit.PlainClone(clonePath, false, &gogit.CloneOptions{
		URL:  repo,
		Auth: auth,
	})
}

func pull(clonePath string, auth *gitssh.PublicKeys) (*gogit.Repository, error) {
	r, err := gogit.PlainOpen(clonePath)
	if err != nil {
		return nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	err = w.Pull(&gogit.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
	})

	if err == gogit.NoErrAlreadyUpToDate {
		return r, nil
	}

	// go-git doesn't have an error variable for "non-fast-forward update", so
	// this is the only way to detect it
	if strings.HasPrefix(err.Error(), "non-fast-forward update") {
		return r, nil
	}

	return r, err
}

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

func Push(r *gogit.Repository, cfg config.GitSettings) error {
	auth, err := getAuth(cfg.User, cfg.PrivateKeyPath)
	if err != nil {
		return err
	}

	err = r.Push(&gogit.PushOptions{
		Auth: auth,
	})

	if err == gogit.NoErrAlreadyUpToDate {
		return nil
	}

	// go-git doesn't have an error variable for "non-fast-forward update", so
	// this is the only way to detect it
	if strings.HasPrefix(err.Error(), "non-fast-forward update") {
		return nil
	}

	return err
}
