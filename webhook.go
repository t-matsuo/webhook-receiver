package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var goenv struct {
	Cmd        string `required:"true"`
	Port       int    `default:"22999"`
	Bind       string `default:"0.0.0.0"`
	Path       string `default:"/"`
	Debug      bool   `default:"false"`
	Log_prefix string `default:"[webhook]"`
	No_alog    bool   `default:"false"`
	Timeout    int    `default:"300"`
	Workdir    string `default:"/tmp"`
}

var log_info *log.Logger
var log_err *log.Logger
var log_debug *log.Logger
var log_access *log.Logger

func handleEnv() {
	if err := envconfig.Process("webhook", &goenv); err != nil {
		log_err.Fatalf("Failed to process env: %s", err)
		os.Exit(1)
	}

	if goenv.Debug == true {
		log_info.SetFlags(log.LstdFlags | log.Llongfile | log.Lmsgprefix)
		log_err.SetFlags(log.LstdFlags | log.Llongfile | log.Lmsgprefix)
		log_debug.SetOutput(os.Stderr)
		log_access.SetFlags(log.LstdFlags | log.Llongfile | log.Lmsgprefix)
	}

	if goenv.No_alog == true {
		log_access.SetOutput(ioutil.Discard)
	}

	log_info.SetPrefix(goenv.Log_prefix + " INFO ")
	log_err.SetPrefix(goenv.Log_prefix + " ERROR ")
	log_debug.SetPrefix(goenv.Log_prefix + " DEBUG ")
	log_access.SetPrefix(goenv.Log_prefix + " ACCESS ")

	_, err := os.Stat(goenv.Cmd)
	if os.IsNotExist(err) {
		log_err.Fatalf("%s command not found\n", goenv.Cmd)
	}
	goenv.Cmd, _ = filepath.Abs(goenv.Cmd)

	if goenv.Port <= 0 || goenv.Port > 65535 {
		log_err.Fatalf("Invalid port number %d\n", goenv.Port)
	}

	if net.ParseIP(goenv.Bind) == nil {
		log_err.Fatalf("Invalid bind IP %s\n", goenv.Bind)
	}

	if !regexp.MustCompile(`^/[0-9A-z/\-]*$`).MatchString(goenv.Path) {
		log_err.Fatalf("Invalid Path %s\n", goenv.Path)
	}

	finfo, err := os.Stat(goenv.Workdir)
	if os.IsNotExist(err) || !finfo.IsDir() {
		log_err.Fatalf("%s directory not found\n", goenv.Workdir)
	}

	log_info.Printf("Command is %s\n", goenv.Cmd)
	log_info.Printf("Workdir is %s\n", goenv.Workdir)
	log_info.Printf("Listening on http://%s:%d%s\n", goenv.Bind, goenv.Port, goenv.Path)
}

func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func handleReq(w http.ResponseWriter, r *http.Request) {
	ip := GetIP(r)
	host := r.Host
	ua := r.UserAgent()
	path := r.URL.Path

	alog_format := ip

	if path != goenv.Path {
		log_access.Printf("%s %s Not Found %s\n", alog_format, r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not Found\n")
		return
	}

	switch r.Method {
	case "POST":
		bufbody := new(bytes.Buffer)
		bufbody.ReadFrom(r.Body)
		body := bufbody.String()
		log_access.Printf("%s %s '%v'\n", alog_format, r.Method, body)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(goenv.Timeout)*time.Second)
		cmd := exec.CommandContext(ctx, goenv.Cmd, body)
		defer cancel()

		cmd.Env = append(os.Environ(),
			"WEBHOOK_IP="+ip,
			"WEBHOOK_HOST="+host,
			"WEBHOOK_UA="+ua,
			"WEBHOOK_PATH="+path,
		)
		cmd.Dir = goenv.Workdir
		err := cmd.Run()

		if err != nil {
			log_err.Printf("Internal Server Error. %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Internal Server Error\n")
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "OK\n")
		}
	default:
		log_access.Printf("%s %s Method Not Allowed\n", alog_format, r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method Not Allowed\n")
	}
}

func init() {
	log_info = log.New(os.Stdout, "[webhook] INFO ", log.LstdFlags|log.Lmsgprefix)
	log_err = log.New(os.Stderr, "[webhook] ERROR ", log.LstdFlags|log.Lmsgprefix)
	log_debug = log.New(ioutil.Discard, "[webhook] DEBUG ", log.LstdFlags|log.Llongfile|log.Lmsgprefix)
	log_access = log.New(os.Stdout, "[webhook] ACCESS ", log.LstdFlags|log.Lmsgprefix)
}

func main() {
	handleEnv()
	http.HandleFunc(goenv.Path, handleReq)

	if err := http.ListenAndServe(goenv.Bind+":"+strconv.Itoa(goenv.Port), nil); err != nil {
		log_err.Fatal(err)
	}
}
