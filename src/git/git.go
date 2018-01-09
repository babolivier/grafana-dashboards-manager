package git

import (
	"io/ioutil"
	"os"
	"strings"

	"config"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

// Sync synchronises a Git repository using a given configuration. "synchronises"
// means that, if the repo from the configuration isn't already cloned in the
// directory specified in the configuration, it will clone the repository,
// else it will simply pull it in order to be up to date with the remote.
// Returns the go-git representation of the repository.
// Returns an error if there was an issue loading the SSH private key, checking
// whether the clone path already exists, or synchronising the repo with the
// remote.
func Sync(cfg config.GitSettings) (r *gogit.Repository, err error) {
	// Generate an authentication structure instance from the user and private
	// key
	auth, err := getAuth(cfg.User, cfg.PrivateKeyPath)
	if err != nil {
		return
	}

	// Check whether the clone path already exists
	exists, err := dirExists(cfg.ClonePath)
	if err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{
		"repo":       cfg.User + "@" + cfg.URL,
		"clone_path": cfg.ClonePath,
		"pull":       exists,
	}).Info("Synchronising the Git repository with the remote")

	// If the clone path already exists, pull from the remote, else clone it.
	if exists {
		r, err = pull(cfg.ClonePath, auth)
	} else {
		r, err = clone(cfg.URL, cfg.ClonePath, auth)
	}

	return
}

// getAuth returns the authentication structure instance needed to authenticate
// on the remote, using a given user and private key path.
// Returns an error if there was an issue reading the private key file or
// parsing it.
func getAuth(user string, privateKeyPath string) (*gitssh.PublicKeys, error) {
	privateKey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return &gitssh.PublicKeys{User: user, Signer: signer}, nil
}

// clone clones a Git repository into a given path, using a given auth.
// Returns the go-git representation of the Git repository.
// Returns an error if there was an issue cloning the repository.
func clone(repo string, clonePath string, auth *gitssh.PublicKeys) (*gogit.Repository, error) {
	return gogit.PlainClone(clonePath, false, &gogit.CloneOptions{
		URL:  repo,
		Auth: auth,
	})
}

// pull opens the repository located at a given path, and pulls it from the
// remote using a given auth, in order to be up to date with the remote.
// Returns with the go-git representation of the repository.
// Returns an error if there was an issue opening the repo, getting its work
// tree or pulling from the remote. In the latter case, if the error is "already
// up to date", "non-fast-forward update" or "remote repository is empty",
// doesn't return any error.
func pull(clonePath string, auth *gitssh.PublicKeys) (*gogit.Repository, error) {
	// Open the repository
	r, err := gogit.PlainOpen(clonePath)
	if err != nil {
		return nil, err
	}

	// Get its worktree
	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	// Pull from remote
	if err = w.Pull(&gogit.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
	}); err != nil {
		// Check error against known non-errors
		err = checkRemoteErrors(err, logrus.Fields{
			"clone_path": clonePath,
			"error":      err,
		})
	}

	return r, err
}

// dirExists is a snippet checking if a directory exists on the disk.
// Returns with a boolean set to true if the directory exists, false if not.
// Returns with an error if there was an issue checking the directory's
// existence.
func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

// Push uses a given repository and configuration to push the local history of
// the said repository to the remote, using an authentication structure instance
// created from the configuration to authenticate on the remote.
// Returns with an error if there was an issue creating the authentication
// structure instance or pushing to the remote. In the latter case, if the error
// is "already up to date", "non-fast-forward update" or "remote repository is
// empty", doesn't return any error.
func Push(r *gogit.Repository, cfg config.GitSettings) error {
	// Get the authentication structure instance
	auth, err := getAuth(cfg.User, cfg.PrivateKeyPath)
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"repo":       cfg.User + "@" + cfg.URL,
		"clone_path": cfg.ClonePath,
	}).Info("Pushing to the remote")

	// Push to remote
	if err = r.Push(&gogit.PushOptions{
		Auth: auth,
	}); err != nil {
		// Check error against known non-errors
		err = checkRemoteErrors(err, logrus.Fields{
			"repo":       cfg.User + "@" + cfg.URL,
			"clone_path": cfg.ClonePath,
			"error":      err,
		})
	}

	return err
}

// processRemoteErrors checks an error against known non-errors returned when
// communicating with the remote. If the error is a non-error, returns nil and
// logs it with the provided fields. If not, returns the error.
func checkRemoteErrors(err error, logFields logrus.Fields) error {
	var nonError bool

	// Check against known non-errors
	switch err {
	case gogit.NoErrAlreadyUpToDate:
		nonError = true
		break
	case transport.ErrEmptyRemoteRepository:
		nonError = true
		break
	default:
		nonError = false
		break
	}

	// go-git doesn't have an error variable for "non-fast-forward update", so
	// this is the only way to detect it
	if strings.HasPrefix(err.Error(), "non-fast-forward update") {
		nonError = true
	}

	// Log non-error
	if nonError {
		logrus.WithFields(logFields).Warn("Caught specific non-error")

		return nil
	}

	return err
}
