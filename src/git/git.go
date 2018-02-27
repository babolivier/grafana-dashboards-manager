package git

import (
	"fmt"
	"io/ioutil"
	"os"

	"config"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

// Repository represents a Git repository, as an abstraction layer above the
// go-git library in order to also store the current configuration and the
// authentication data needed to talk to the Git remote.
type Repository struct {
	Repo *gogit.Repository
	cfg  *config.GitSettings
	auth *gitssh.PublicKeys
}

// NewRepository creates a new instance of the Repository structure and fills
// it accordingly to the current configuration.
// Returns a boolean if the clone path doesn't contain a valid Git repository
// and needs the repository to be cloned from remote before it is usable.
// Returns an error if there was an issue opening the clone path or loading
// authentication data.
func NewRepository(cfg *config.GitSettings) (r *Repository, invalidRepo bool, err error) {
	// Load the repository.
	repo, err := gogit.PlainOpen(cfg.ClonePath)
	if err != nil {
		if err == gogit.ErrRepositoryNotExists {
			invalidRepo = true
		} else {
			return
		}
	}

	// Fill the structure instance with the gogit.Repository instance and the
	// configuration.
	r = &Repository{
		Repo: repo,
		cfg:  cfg,
	}

	// Load authentication data in the structure instance.
	err = r.getAuth()
	return
}

// Sync synchronises a Git repository using a given configuration. "synchronises"
// means that, if the repo from the configuration isn't already cloned in the
// directory specified in the configuration, it will clone the repository (unless
// if explicitely told not to), else it will simply pull it in order to be up to
// date with the remote.
// Returns the go-git representation of the repository.
// Returns an error if there was an issue loading the SSH private key, checking
// whether the clone path already exists, or synchronising the repo with the
// remote.
func (r *Repository) Sync(dontClone bool) (err error) {
	// Check whether the clone path already exists.
	exists, err := dirExists(r.cfg.ClonePath)
	if err != nil {
		return
	}

	// Check whether the clone path is a Git repository.
	var isRepo bool
	if isRepo, err = dirExists(r.cfg.ClonePath + "/.git"); err != nil {
		return
	} else if exists && !isRepo {
		err = fmt.Errorf(
			"%s already exists but is not a Git repository",
			r.cfg.ClonePath,
		)

		return
	}

	logrus.WithFields(logrus.Fields{
		"repo":       r.cfg.User + "@" + r.cfg.URL,
		"clone_path": r.cfg.ClonePath,
		"pull":       exists,
	}).Info("Synchronising the Git repository with the remote")

	// If the clone path already exists, pull from the remote, else clone it.
	if exists {
		err = r.pull()
	} else if !dontClone {
		err = r.clone()
	}

	return
}

// Push uses a given repository and configuration to push the local history of
// the said repository to the remote, using an authentication structure instance
// created from the configuration to authenticate on the remote.
// Returns with an error if there was an issue creating the authentication
// structure instance or pushing to the remote. In the latter case, if the error
// is a known non-error, doesn't return any error.
func (r *Repository) Push() (err error) {
	logrus.WithFields(logrus.Fields{
		"repo":       r.cfg.User + "@" + r.cfg.URL,
		"clone_path": r.cfg.ClonePath,
	}).Info("Pushing to the remote")

	// Push to remote.
	if err = r.Repo.Push(&gogit.PushOptions{
		Auth: r.auth,
	}); err != nil {
		// Check error against known non-errors.
		err = checkRemoteErrors(err, logrus.Fields{
			"repo":       r.cfg.User + "@" + r.cfg.URL,
			"clone_path": r.cfg.ClonePath,
			"error":      err,
		})
	}

	return err
}

// GetLatestCommit retrieves the latest commit from the local Git repository and
// returns it.
// Returns an error if there was an issue fetching the references or loading the
// latest one.
func (r *Repository) GetLatestCommit() (*object.Commit, error) {
	// Retrieve the list of references from the repository.
	refs, err := r.Repo.References()
	if err != nil {
		return nil, err
	}

	// Extract the latest reference.
	ref, err := refs.Next()
	if err != nil {
		return nil, err
	}

	// Load the commit matching the reference's hash and return it.
	hash := ref.Hash()
	return r.Repo.CommitObject(hash)
}

// Log loads the Git repository's log, with the most recent commit having the
// given hash.
// Returns an error if the log couldn't be loaded.
func (r *Repository) Log(fromHash string) (object.CommitIter, error) {
	hash := plumbing.NewHash(fromHash)

	return r.Repo.Log(&gogit.LogOptions{
		From: hash,
	})
}

// GetModifiedAndRemovedFiles takes to commits and returns the name of files
// that were added, modified or removed between these two commits. Note that
// the added/modified files and the removed files are returned in two separated
// slices, mainly because some features using this function need to load the
// files' contents afterwards, and this is done differently depending on whether
// the file was removed or not.
// "from" refers to the oldest commit of both, and "to" to the latest one.
// Returns empty slices and no error if both commits have the same hash.
// Returns an error if there was an issue loading the repository's log, the
// commits' stats, or retrieving a file from the repository.
func (r *Repository) GetModifiedAndRemovedFiles(
	from *object.Commit, to *object.Commit,
) (modified []string, removed []string, err error) {
	// Initialise the slices.
	modified = make([]string, 0)
	removed = make([]string, 0)

	// We expect "from" to be the oldest commit, and "to" to be the most recent
	// one. Because Log() works the other way (in anti-chronological order),
	// we call it with "to" and not "from" because, that way, we'll go from "to"
	// to "from".
	iter, err := r.Log(to.Hash.String())
	if err != nil {
		return
	}

	// Iterate over the commits contained in the commit's log.
	err = iter.ForEach(func(commit *object.Commit) error {
		// If the commit was done by the manager, go to the next iteration.
		if commit.Author.Email == r.cfg.CommitsAuthor.Email {
			return nil
		}

		// If the current commit is the oldest one requested, break the loop.
		if commit.Hash.String() == from.Hash.String() {
			return storer.ErrStop
		}

		// Load stats from the current commit.
		stats, err := commit.Stats()
		if err != nil {
			return err
		}

		// Iterate over the files contained in the commit's stats.
		for _, stat := range stats {
			// Try to access the file's content.
			_, err := commit.File(stat.Name)
			if err != nil && err != object.ErrFileNotFound {
				return err
			}

			// If the content couldn't be retrieved, it means the file was
			// removed in this commit, else it means that it was either added or
			// modified.
			if err == object.ErrFileNotFound {
				removed = append(removed, stat.Name)
			} else {
				modified = append(modified, stat.Name)
			}
		}

		return nil
	})

	return
}

// GetFilesContentsAtCommit retrieves the state of the repository at a given
// commit, and returns a map contaning the contents of all files in the repository
// at this time.
// Returns an error if there was an issue loading the commit's tree, or loading
// a file's content.
func (r *Repository) GetFilesContentsAtCommit(commit *object.Commit) (map[string][]byte, error) {
	var content string

	// Load the commit's tree.
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	// Initialise the map that will be returned.
	filesContents := make(map[string][]byte)
	// Load the files from the tree.
	files := tree.Files()

	// Iterate over the files.
	err = files.ForEach(func(file *object.File) error {
		// Try to access the file's content at the given commit.
		content, err = file.Contents()
		if err != nil {
			return err
		}

		// Append the content to the map.
		filesContents[file.Name] = []byte(content)

		return nil
	})

	return filesContents, err
}

// getAuth returns the authentication structure instance needed to authenticate
// on the remote, using a given user and private key path.
// Returns an error if there was an issue reading the private key file or
// parsing it.
func (r *Repository) getAuth() error {
	// Load the private key.
	privateKey, err := ioutil.ReadFile(r.cfg.PrivateKeyPath)
	if err != nil {
		return err
	}

	// Parse the private key.
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return err
	}

	r.auth = &gitssh.PublicKeys{User: r.cfg.User, Signer: signer}
	return nil
}

// clone clones a Git repository into a given path, using a given auth.
// Returns the go-git representation of the Git repository.
// Returns an error if there was an issue cloning the repository.
func (r *Repository) clone() (err error) {
	r.Repo, err = gogit.PlainClone(r.cfg.ClonePath, false, &gogit.CloneOptions{
		URL:  r.cfg.URL,
		Auth: r.auth,
	})

	return err
}

// pull opens the repository located at a given path, and pulls it from the
// remote using a given auth, in order to be up to date with the remote.
// Returns with the go-git representation of the repository.
// Returns an error if there was an issue opening the repo, getting its work
// tree or pulling from the remote. In the latter case, if the error is a known
// non-error, doesn't return any error.
func (r *Repository) pull() error {
	// Open the repository.
	repo, err := gogit.PlainOpen(r.cfg.ClonePath)
	if err != nil {
		return err
	}

	// Get its worktree.
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Pull from remote.
	if err = w.Pull(&gogit.PullOptions{
		RemoteName: "origin",
		Auth:       r.auth,
	}); err != nil {
		// Check error against known non-errors.
		err = checkRemoteErrors(err, logrus.Fields{
			"clone_path": r.cfg.ClonePath,
			"error":      err,
		})
	}

	r.Repo = repo

	return err
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

// processRemoteErrors checks an error against known non-errors returned when
// communicating with the remote. If the error is a non-error, returns nil and
// logs it with the provided fields. If not, returns the error.
// Current known non-errors are "already up to date" and "remote repository is
// empty".
func checkRemoteErrors(err error, logFields logrus.Fields) error {
	var nonError bool

	// Check against known non-errors.
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

	// Log non-error.
	if nonError {
		logrus.WithFields(logFields).Warn("Caught specific non-error")

		return nil
	}

	return err
}
