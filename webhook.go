package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

var goenv struct {
	Cmd  string `required:"true"`
	Port int    `default:"22999"`
	Bind string `default:"0.0.0.0"`
	Path string `default:"/"`
}

func handleEnv() {
	if err := envconfig.Process("webhook", &goenv); err != nil {
		fmt.Printf("ERROR: Failed to process env: %s", err)
		os.Exit(1)
	}

	_, err := os.Stat(goenv.Cmd)
	if os.IsNotExist(err) {
		fmt.Printf("ERROR: %s command not found\n", goenv.Cmd)
		os.Exit(1)
	}

	if goenv.Port <= 0 || goenv.Port > 65535 {
		fmt.Printf("ERROR: Invalid port number %d\n", goenv.Port)
		os.Exit(1)
	}

	if net.ParseIP(goenv.Bind) == nil {
		fmt.Println("ERROR: Invalid bind IP " + goenv.Bind)
		os.Exit(1)
	}

	if !regexp.MustCompile(`^/[0-9A-z/\-]*$`).MatchString(goenv.Path) {
		fmt.Println("ERROR: Invalid Path " + goenv.Path)
		os.Exit(1)
	}

	script := os.Getenv("WEBHOOK_CMD")
	fmt.Println("Command: " + script)
	fmt.Printf("Webhook listening at: http://%s:%d%s\n", goenv.Bind, goenv.Port, goenv.Path)
}

func handleReq(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != goenv.Path {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "POST":
		bufbody := new(bytes.Buffer)
		bufbody.ReadFrom(r.Body)
		body := bufbody.String()
		if body == "" {
			fmt.Print("POST body is empty")
			return
		}

		fmt.Printf("POST body : '%v'\n", body)
		exec.Command(goenv.Cmd, body).Start()
	default:
		fmt.Fprintf(w, "Invalid method.\n")
	}
}

func main() {
	handleEnv()
	http.HandleFunc(goenv.Path, handleReq)

	if err := http.ListenAndServe(goenv.Bind+":"+strconv.Itoa(goenv.Port), nil); err != nil {
		log.Fatal(err)
	}
}
