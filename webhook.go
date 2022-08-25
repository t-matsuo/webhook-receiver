package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
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
	Cmd           string `required:"true"`
	Output_stdout bool   `default:"false"`
	Output_stderr bool   `default:"false"`
	Port          int    `default:"22999"`
	Bind          string `default:"0.0.0.0"`
	Path          string `default:"/"`
	Debug         bool   `default:"false"`
	Log_prefix    string `default:"[webhook]"`
	No_alog       bool   `default:"false"`
	Timeout       int    `default:"300"`
	Workdir       string `default:"/tmp"`
	Tls           bool   `default:"false"`
	Server_crt    string `default:""`
	Server_key    string `default:""`
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

	finfo_workdir, err := os.Stat(goenv.Workdir)
	if os.IsNotExist(err) || !finfo_workdir.IsDir() {
		log_err.Fatalf("%s directory not found\n", goenv.Workdir)
	}

	if goenv.Tls == true && (goenv.Server_crt != "" || goenv.Server_key != "") {
		finfo_crt, err := os.Stat(goenv.Server_crt)
		if os.IsNotExist(err) || finfo_crt.IsDir() {
			log_err.Fatalf("TLS %s crt file not found\n", goenv.Server_crt)
		}
		finfo_key, err := os.Stat(goenv.Server_key)
		if os.IsNotExist(err) || finfo_key.IsDir() {
			log_err.Fatalf("TLS %s key file not found\n", goenv.Server_key)
		}
	}

	log_info.Printf("Command is %s\n", goenv.Cmd)
	log_info.Printf("Output stdout is %t\n", goenv.Output_stdout)
	log_info.Printf("Output stderr is %t\n", goenv.Output_stderr)
	log_info.Printf("Workdir is %s\n", goenv.Workdir)
	if goenv.Tls == false {
		log_info.Printf("Listening on http://%s:%d%s\n", goenv.Bind, goenv.Port, goenv.Path)
	} else {
		if goenv.Server_crt != "" || goenv.Server_key != "" {
			log_info.Printf("TLS Server crt file is %s", goenv.Server_crt)
			log_info.Printf("TLS Server key file is %s", goenv.Server_key)
		}
		log_info.Printf("Listening on https://%s:%d%s\n", goenv.Bind, goenv.Port, goenv.Path)
	}
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

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()

		if err != nil {
			log_err.Printf("Internal Server Error. %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Internal Server Error\n")
			if goenv.Output_stderr {
				fmt.Fprintf(w, "\n############### stderr ###############\n%s\n", stderr.String())
			}
			if goenv.Output_stdout {
				fmt.Fprintf(w, "\n############### stdout ###############\n%s\n", stdout.String())
			}
		} else {
			w.WriteHeader(http.StatusOK)
			if goenv.Output_stdout {
				fmt.Fprintf(w, "%s", stdout.String())
			} else {
				fmt.Fprintf(w, "OK\n")
			}
		}
	default:
		log_access.Printf("%s %s Method Not Allowed\n", alog_format, r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method Not Allowed\n")
	}
}

func genTlsKeyPair() (tls.Certificate, error) {
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName:         "Fake Certificate for Webhook",
			Country:            []string{"Fake Certificate for Webhook"},
			Organization:       []string{"Fake Certificate for Webhook"},
			OrganizationalUnit: []string{"Fake Certificate for Webhook"},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(10, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert, err := x509.CreateCertificate(rand.Reader, template, template,
		priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	var outCert tls.Certificate
	outCert.Certificate = append(outCert.Certificate, cert)
	outCert.PrivateKey = priv

	return outCert, nil
}

func listenAndServeTLSKeyPair(addr string, cert tls.Certificate, handler http.Handler) error {
	if addr == "" {
		addr = ":https"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	return server.ServeTLS(ln, "", "")
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

	if goenv.Tls == false {
		if err := http.ListenAndServe(goenv.Bind+":"+strconv.Itoa(goenv.Port), nil); err != nil {
			log_err.Fatal(err)
		}
	} else {
		if goenv.Server_crt == "" && goenv.Server_key == "" {
			if cert, err := genTlsKeyPair(); err != nil {
				log_err.Fatal(err)
			} else {
				if err := listenAndServeTLSKeyPair(goenv.Bind+":"+strconv.Itoa(goenv.Port), cert, nil); err != nil {
					log_err.Fatal(err)
				}
			}
		} else {
			if err := http.ListenAndServeTLS(goenv.Bind+":"+strconv.Itoa(goenv.Port), goenv.Server_crt, goenv.Server_key, nil); err != nil {
				log_err.Fatal(err)
			}
		}
	}
}
