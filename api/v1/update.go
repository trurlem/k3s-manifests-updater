package v1

import (
	"fmt"
	"net/http"

	"github.com/trurlem/k3s-manifests-updater/config"
	"github.com/trurlem/k3s-manifests-updater/filesystem"
)

func HandleUpdate(conf config.Config, w http.ResponseWriter) {
	s := filesystem.NewGitSyncer(conf.GitRepoURL)
	err := s.Sync()
	if err != nil {
		fmt.Println("Error updating manifests:", err)
	}
	w.WriteHeader(http.StatusOK)
}
