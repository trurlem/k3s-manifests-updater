package filesystem

import (
	"crypto"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

var manifestIgnoreList = []string{
	"ccm.yaml",
	"coredns.yaml",
	"local-storage.yaml",
	"metrics-server/",
	"rolebindings.yaml",
	"traefik.yaml",
	"metrics-server",
}

const manifestsDir = "/var/lib/rancher/k3s/server/manifests/"

var manifestValidSuffixes = []string{
	".yaml",
	".yml",
	".json",
}

type Syncer interface {
	Sync() error
}

type manifestData struct {
	Path     string
	Size     int64
	Hash     [20]byte // a sha1 hash
	Contents []byte   // not always set (TODO: refactor this)
}

type gitSyncer struct {
	gitRepoURL      string
	fsManifestsDir  string
	ignoreList      []string
	validSuffixList []string
}

func NewGitSyncer(gitRepoURL string) *gitSyncer {
	return &gitSyncer{
		gitRepoURL:      gitRepoURL,
		fsManifestsDir:  manifestsDir,
		ignoreList:      manifestIgnoreList,
		validSuffixList: manifestValidSuffixes,
	}
}

func sha1FromFileContents(contents string) (hashValue [20]byte) {
	h := crypto.SHA1.New()
	h.Write([]byte(contents))
	copy(hashValue[:], h.Sum(nil))

	return hashValue
}

func (s *gitSyncer) manifestsToDeploy() (manifests map[string]*manifestData, err error) {
	storage := memory.NewStorage()
	storage.IterReferences()
	gitRepo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: s.gitRepoURL,
	})
	if err != nil {
		return nil, fmt.Errorf("clone failed: %w", err)
	}

	gitHead, err := gitRepo.Head()
	if err != nil {
		return nil, fmt.Errorf("head failed: %w", err)
	}

	commit, err := gitRepo.CommitObject(gitHead.Hash())
	if err != nil {
		return nil, fmt.Errorf("fetching commit by hash failed: %w", err)
	}

	fileIter, err := commit.Files()
	if err != nil {
		return nil, fmt.Errorf("extracting file iterator from commit failed: %w", err)
	}

	manifests = make(map[string]*manifestData)
	fileIter.ForEach(func(f *object.File) error {
		for _, prefix := range s.ignoreList {
			if strings.HasPrefix(f.Name, prefix) {
				return nil
			}
		}

		for _, suffix := range s.validSuffixList {
			if strings.HasSuffix(f.Name, suffix) {
				break
			}
			return nil
		}

		fileContents, err := f.Contents()
		if err != nil {
			return nil
		}

		manifests[f.Name] = &manifestData{
			Path:     f.Name,
			Size:     f.Size,
			Hash:     sha1FromFileContents(fileContents),
			Contents: []byte(fileContents),
		}

		return nil
	})

	return manifests, nil
}

func sha1FromFilePath(path string) (hashValue [20]byte) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sha1 := crypto.SHA1.New()
	if _, err := io.Copy(sha1, f); err != nil {
		return
	}

	copy(hashValue[:], sha1.Sum(nil))

	return hashValue
}

// TODO: copy safely
// TODO: adjust perms if we run as root
func (s *gitSyncer) copyManifest(path string, contents []byte) error {
	localPath := filepath.Join(s.fsManifestsDir, path)
	if err := os.MkdirAll(filepath.Dir(localPath), 0744); err != nil {
		return err
	}
	return os.WriteFile(localPath, contents, 0644)
}

// TODO: clean up stale empty folders
func (s *gitSyncer) removeManifest(path string) error {
	localPath := filepath.Join(s.fsManifestsDir, path)
	return os.Remove(localPath)
}

func (s *gitSyncer) Sync() (err error) {
	fmt.Println("Syncer: starting sync")

	toDeploy, err := s.manifestsToDeploy()
	if err != nil {
		return fmt.Errorf("fetching manifests to deploy failed: %w", err)
	}

	deployed := make(map[string]*manifestData)
	if err = filepath.Walk(s.fsManifestsDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		trimedPath := strings.TrimPrefix(path, s.fsManifestsDir)
		for _, prefix := range s.ignoreList {
			if strings.HasPrefix(trimedPath, prefix) {
				return nil
			}
		}
		if trimedPath == "" {
			return nil
		}
		deployed[trimedPath] = &manifestData{
			Path: trimedPath,
			Size: info.Size(),
			Hash: sha1FromFilePath(path),
		}
		return nil
	}); err != nil {
		return err
	}

	for path, toDeployManifest := range toDeploy {
		deployedManifest, exists := deployed[path]
		if exists {
			fmt.Println("Comparing contents and copy if needed:", path)
			fmt.Println("Deployed:", deployedManifest.Path, deployedManifest.Hash)
			fmt.Println("To Deploy:", toDeployManifest.Path, deployedManifest.Hash)
			fmt.Println("Is it the same?", deployedManifest.Hash == toDeployManifest.Hash)
			if deployedManifest.Hash == toDeployManifest.Hash {
				fmt.Println("Skipping copy for", toDeployManifest.Path)
				continue
			}
			fmt.Println("Copying new version of file:", toDeployManifest.Path)
			if err = s.copyManifest(path, toDeployManifest.Contents); err != nil {
				fmt.Println("Could not copy manifest", path, "due to:", err)
			}
		} else {
			fmt.Println("To Deploy:", toDeployManifest.Path, toDeployManifest.Hash)
			fmt.Println("Copying new file", path)
			if err = s.copyManifest(path, toDeployManifest.Contents); err != nil {
				fmt.Println("Could not copy manifest", path, "due to:", err)
			}
		}
		fmt.Println("---------------")
	}

	for path, deployedManifest := range deployed {
		_, exists := toDeploy[path]
		if !exists {
			fmt.Println("Delete manifest: ", path)
			fmt.Println("Deployed:", deployedManifest)
			s.removeManifest(path)
		}
	}

	fmt.Println("Syncer: ending sync")

	return nil
}
