package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultLoadTime = 10 * time.Second
	defaultMemory   = 1
	MB              = 1024 * 1024
)

var loads []string

type loadInstance struct {
	memory   int
	duration time.Duration
}

var mu sync.Mutex
var hostname = ""

func main() {
	fchan := make(chan loadInstance)
	hostname, _ = os.Hostname()
	http.HandleFunc("/memLoad", newMemLoad(fchan))
	http.HandleFunc("/curLoad", curLoad)
	http.HandleFunc("/cpuLoad", newCPULoad(fchan))
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go loadFinishChecker(fchan)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func loadFinishChecker(fchan chan loadInstance) {
	for instance := range fchan {
		fmt.Printf("MemLoad request finished for %d MB memory for %d seconds\n", instance.memory, instance.duration/time.Second)
		mu.Lock()
		for i := 0; i < len(loads); i++ {
			if len(loads[i]) == instance.memory*MB {
				loads = append(loads[:i], loads[i+1:]...)
				break
			}
		}
		runtime.GC()
		mu.Unlock()
	}
}

func curLoad(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	w.Write([]byte(fmt.Sprintf("[%s] Current memory load opeartions: %d\n", hostname, len(loads))))
	for i := 0; i < len(loads); i++ {
		w.Write([]byte(fmt.Sprintf("#%d: %dMB\n", i, len(loads[i])/MB)))
	}
	w.WriteHeader(http.StatusOK)
}

func newMemLoad(fchan chan loadInstance) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		loadTime := defaultLoadTime
		loadMemory := defaultMemory
		rawTime := r.URL.Query().Get("time")
		if rawTime != "" {
			newTime, err := strconv.Atoi(rawTime)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("invalid `time` query param format"))
				return
			} else {
				loadTime = time.Duration(newTime) * time.Second
			}
		}
		rawMem := r.URL.Query().Get("mem")
		if rawMem != "" {
			newMem, err := strconv.Atoi(rawMem)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("invalid `mem` query param format"))
				return
			} else {
				loadMemory = newMem
			}
		}

		fmt.Printf("[%s] MemLoad request for %d MB memory for %d seconds\n", hostname, loadMemory, loadTime/time.Second)
		w.Write([]byte(fmt.Sprintf("[%s] MemLoad request for %d MB memory for %d seconds\n", hostname, loadMemory, loadTime/time.Second)))
		w.WriteHeader(http.StatusOK)
		go makeLoadMemory(fchan, loadMemory, loadTime)
	}
}

var mu1 sync.Mutex

func newCPULoad(fchan chan loadInstance) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		loadTime := 10 * time.Second
		rawTime := r.URL.Query().Get("time")
		if rawTime != "" {
			newTime, err := strconv.Atoi(rawTime)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("invalid `time` query param format"))
				return
			} else {
				loadTime = time.Duration(newTime) * time.Second
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("[%s] CPULoad request for %d seconds\n", hostname, loadTime/time.Second)))
		go func(timout *time.Timer) {
			mu1.Lock()
			defer mu1.Unlock()
			f, err := os.Open(os.DevNull)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			for {
				select {
				case <-timout.C:
					return
				default:
					fmt.Fprintf(f, ".")
				}
			}
		}(time.NewTimer(loadTime))
	}
}

func makeLoadMemory(fChan chan loadInstance, m int, t time.Duration) {
	var b strings.Builder
	b.Grow(m * MB)
	for i := 0; i < m*MB; i++ {
		b.WriteByte(0)
	}
	s := b.String()
	mu.Lock()
	loads = append(loads, s)
	mu.Unlock()
	time.Sleep(t)
	fChan <- loadInstance{
		memory:   m,
		duration: t,
	}
}
