package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/JeffResc/LazyDocker/lib"
)

func main() {
	enabled, err := strconv.ParseBool(os.Getenv("ENABLED"))

	if err != nil {
		panic(err)
	}

	if enabled {
		lib.Init()

		http.HandleFunc("/thaw", thawContainer)
		http.HandleFunc("/reload", reloadConfiguration)

		http.Handle("/lazydocker/", http.StripPrefix("/lazydocker", http.FileServer(http.Dir("/app/pages/static/"))))
		http.ListenAndServe(":80", nil)
	} else {
		fmt.Print("Container not enabled, doing nothing...\nSet environment variable \"ENABLED=true\" if you wish to enable this container.")
	}
}

func reloadConfiguration(w http.ResponseWriter, r *http.Request) {
	lib.PopulateLazyContainers()
	fmt.Fprintf(w, "Configuration reloaded.")
}

func thawContainer(w http.ResponseWriter, r *http.Request) {
	// Check for name parameter
	query := r.URL.Query()
	names, present := query["name"]
	if !present || len(names) == 0 {
		w.WriteHeader(418)
		fmt.Fprintf(w, "name parameter not provided.")
		return
	}

	// List of affected container(s)
	containers := lib.LookupLazyContainersByName(names)

	// Loop through states to determine if containers are online
	containersOnline := true
	for i := range containers {
		state := lib.GetContainerState(containers[i])

		containersOnline = containersOnline && state.Running
	}

	// Act depending on overall status
	if containersOnline {
		// Restart timer(s)
		for i := range containers {
			containers[i].ResetTimer()
		}

		// Allow user to access container
		w.WriteHeader(http.StatusOK)
		return
	} else {
		// Start container, add timer
		w.WriteHeader(418)
		http.ServeFile(w, r, "/app/pages/4.html")

		for i := range containers {
			containers[i].ResetTimer()
			containers[i].ThawContainer()
		}
		return
	}
}
