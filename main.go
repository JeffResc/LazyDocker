package main

import (
	"fmt"
	"io/ioutil"
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

		switch containers[i].FreezeMethod {
		case "stop":
			containersOnline = containersOnline && state.Running
		case "pause":
			containersOnline = containersOnline && state.Running && !state.Paused
		default:
			fmt.Printf("[%s] freeze method \"%s\" is invalid.\n", containers[i].Name, containers[i].FreezeMethod)
			return
		}
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
		// Set page style
		var styleVariant int
		if os.Getenv("STYLE_VARIANT") != "" {
			i, err := strconv.Atoi(os.Getenv("STYLE_VARIANT"))
			if err == nil && i >= 1 && i <= 12 {
				styleVariant = i
			} else {
				fmt.Fprintf(w, "Provided STYLE_VARIANT environment variable is invalid. Defaulting to 1.")
				styleVariant = 1
			}
		} else {
			styleVariant = 1
		}

		// Write page
		fileBytes, err := ioutil.ReadFile("/app/pages/" + strconv.Itoa(styleVariant) + ".html")
		if err != nil {
			panic(err)
		}

		w.WriteHeader(418)
		w.Write(fileBytes)

		// Start container, add timer
		for i := range containers {
			containers[i].ResetTimer()
			containers[i].ThawContainer()
		}
		return
	}
}
