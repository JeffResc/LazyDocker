package lib

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var dockerClient *client.Client

var lazyContainers []LazyContainer

type LazyContainer struct {
	FreezeMethod     string
	TimeBeforeFreeze time.Duration
	Container        types.Container
	Name             string
	Timer            *time.Timer
}

func (lc *LazyContainer) ResetTimer() {
	if lc.Timer == nil {
		lc.Timer = time.AfterFunc(lc.TimeBeforeFreeze, lc.FreezeContainer)
	} else {
		//fmt.Printf("[%s] Extending by %s\n", lc.Name, lc.TimeBeforeFreeze)
		if !lc.Timer.Stop() {
			<-lc.Timer.C
		}
		lc.Timer.Reset(lc.TimeBeforeFreeze)
	}
}

func (lc *LazyContainer) ThawContainer() {
	fmt.Printf("[%s] Thawing...\n", lc.Name)
	switch lc.FreezeMethod {
	case "stop":
		err := dockerClient.ContainerStart(context.Background(), lc.Container.ID, types.ContainerStartOptions{})
		if err != nil {
			panic(err)
		}
	case "pause":
		err := dockerClient.ContainerUnpause(context.Background(), lc.Container.ID)
		if err != nil {
			panic(err)
		}
	default:
		fmt.Printf("[%s] freeze method \"%s\" is invalid. Doing nothing...\n", lc.Name, lc.FreezeMethod)
		return
	}
	fmt.Printf("[%s] Thawed!\n", lc.Name)
}

func (lc *LazyContainer) FreezeContainer() {
	lc.Timer = nil
	fmt.Printf("[%s] %s has elapsed, freezing container...\n", lc.Name, lc.TimeBeforeFreeze.String())
	switch lc.FreezeMethod {
	case "stop":
		err := dockerClient.ContainerStop(context.Background(), lc.Container.ID, nil)
		if err != nil {
			panic(err)
		}
	case "pause":
		err := dockerClient.ContainerPause(context.Background(), lc.Container.ID)
		if err != nil {
			panic(err)
		}
	default:
		fmt.Printf("[%s] freeze method \"%s\" is invalid. Doing nothing...\n", lc.Name, lc.FreezeMethod)
		return
	}
	fmt.Printf("[%s] Frozen!\n", lc.Name)
}

func Init() {
	// Create docker client
	var err error
	dockerClient, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	// Populate container options
	PopulateLazyContainers()
}

func LookupLazyContainersByName(names []string) []*LazyContainer {
	var foundContainers []*LazyContainer

	for i := range lazyContainers {
		for j := range names {
			if lazyContainers[i].Name == names[j] {
				foundContainers = append(foundContainers, &lazyContainers[i])
			}
		}
	}

	return foundContainers
}

func PopulateLazyContainers() {
	lazyContainers = nil

	// Get containers
	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{All: true})

	if err != nil {
		panic(err)
	}

	for i := range containers {
		var containerName string
		lc := LazyContainer{}
		if containers[i].Labels["lazydocker.enable"] != "" {
			enabled, err := strconv.ParseBool(containers[i].Labels["lazydocker.enable"])

			if err != nil {
				panic(err)
			}

			if enabled {
				// Name
				if containers[i].Labels["lazydocker.name"] != "" {
					containerName = containers[i].Labels["lazydocker.name"]
				} else {
					containerName = strings.Split(strings.Join(containers[i].Names, ""), "/")[1]
				}

				// Freeze method
				switch strings.ToLower(containers[i].Labels["lazydocker.freeze-method"]) {
				case "stop":
					lc.FreezeMethod = "stop"
				case "pause":
					lc.FreezeMethod = "pause"
				default:
					if os.Getenv("DEFAULT_FREEZE_METHOD") != "" {
						lc.FreezeMethod = os.Getenv("DEFAULT_FREEZE_METHOD")
					} else {
						lc.FreezeMethod = "stop"
					}
				}

				// Time before freeze
				var timeBeforeFreeze time.Duration
				var err error
				if containers[i].Labels["lazydocker.time-before-freeze"] != "" {
					timeBeforeFreeze, err = time.ParseDuration(containers[i].Labels["lazydocker.time-before-freeze"])
				} else {
					if os.Getenv("DEFAULT_TIME_BEFORE_FREEZE") != "" {
						timeBeforeFreeze, err = time.ParseDuration(os.Getenv("DEFAULT_TIME_BEFORE_FREEZE"))
					} else {
						timeBeforeFreeze, err = time.ParseDuration("1m")
					}
				}

				if err != nil {
					panic(err)
				}

				lc.TimeBeforeFreeze = timeBeforeFreeze

				lc.Container = containers[i]
				lc.Name = containerName

				fmt.Printf("[\"%s\" configuration loaded] FreezeMethod:%s TimeBeforeFreeze:%s\n", lc.Name, lc.FreezeMethod, lc.TimeBeforeFreeze)

				switch strings.ToLower(os.Getenv("START_ACTION")) {
				case "freeze":
					lc.FreezeContainer()
				case "run":
					lc.ResetTimer()
				default:
					lc.FreezeContainer()
				}

				lazyContainers = append(lazyContainers, lc)
			}
		}
	}
}

func GetContainerState(lc *LazyContainer) types.ContainerState {
	inspection, err := dockerClient.ContainerInspect(context.Background(), lc.Container.ID)

	if err != nil {
		panic(err)
	}

	return *inspection.State
}
