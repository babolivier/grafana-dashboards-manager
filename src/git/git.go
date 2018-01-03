package git

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"golang.org/x/crypto/ssh"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

func Sync(repo string, clonePath string, privateKeyPath string) (r *git.Repository, err error) {
	auth, err := getAuth(privateKeyPath)
	if err != nil {
		return
	}

	exists, err := dirExists(clonePath)
	if err != nil {
		return
	}

	if exists {
		r, err = pull(clonePath, auth)
	} else {
		r, err = clone(repo, clonePath, auth)
	}

	return
}

func getAuth(privateKeyPath string) (*gitssh.PublicKeys, error) {
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

func clone(repo string, clonePath string, auth *gitssh.PublicKeys) (*git.Repository, error) {
	return git.PlainClone(clonePath, false, &git.CloneOptions{
		URL:  repo,
		Auth: auth,
	})
}

func pull(clonePath string, auth *gitssh.PublicKeys) (*git.Repository, error) {
	r, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	err = w.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
	})

	if err == git.NoErrAlreadyUpToDate {
		return r, nil
	}

	// go-git doesn't have an error variable for "non-fast-forward update", so
	// this is the only way to detect it
	if strings.HasPrefix("non-fast-forward update", err.Error()) {
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

func Commit(message string, w *git.Worktree) (plumbing.Hash, error) {
	return w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Grafana Dashboard Manager",
			Email: "grafana@cozycloud.cc",
			When:  time.Now(),
		},
	})
}

func Push(r *git.Repository, keyPath string) error {
	auth, err := getAuth(keyPath)
	if err != nil {
		return err
	}

	err = r.Push(&git.PushOptions{
		Auth: auth,
	})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	return err
}
