package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

var goenv struct {
	Script string `required:"true"`
	Port   int    `default:"22999"`
	Bind   string `default:"0.0.0.0"`
}

func handleEnv() {
	if err := envconfig.Process("webhook", &goenv); err != nil {
		fmt.Printf("ERROR: Failed to process env: %s", err)
		os.Exit(1)
	}

	_, err := os.Stat(goenv.Script)
	if os.IsNotExist(err) {
		fmt.Printf("ERROR: %s file not found\n", goenv.Script)
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

	script := os.Getenv("WEBHOOK_SCRIPT")
	fmt.Println("Script: " + script)
	fmt.Printf("Bind IP:Port: %s:%d\n", goenv.Bind, goenv.Port)
}

func handleReq(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}

		bufbody := new(bytes.Buffer)
		bufbody.ReadFrom(r.Body)
		body := bufbody.String()

		fmt.Printf("%v\n", body)
		exec.Command(goenv.Script, body).Start()
	default:
		fmt.Fprintf(w, "Invalid method.\n")
	}
}

func main() {
	handleEnv()
	http.HandleFunc("/", handleReq)

	if err := http.ListenAndServe(goenv.Bind+":"+strconv.Itoa(goenv.Port), nil); err != nil {
		log.Fatal(err)
	}
}
