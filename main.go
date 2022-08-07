package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	v1 "github.com/trurlem/k3s-manifests-updater/api/v1"
	"github.com/trurlem/k3s-manifests-updater/config"
)

func setupHandlers(client *http.Client, conf config.Config) *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc(
		"/api/v1/update",
		func(w http.ResponseWriter, _ *http.Request) {
			v1.HandleUpdate(conf, w)
		},
	).Methods(http.MethodPost)

	return r
}

func main() {
	conf, err := config.FromEnv()

	if err != nil {
		panic("Invalid configuration: " + err.Error())
	}

	r := setupHandlers(http.DefaultClient, conf)

	s := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf(":%d", conf.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	hostname, _ := os.Hostname()
	fmt.Println("----------------------")
	fmt.Printf("Starting up server on http://%s.local:%d\n", hostname, conf.Port)
	fmt.Println("----------------------")

	log.Fatal(s.ListenAndServe())
}
