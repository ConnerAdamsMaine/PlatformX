package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/404cnf/meshgrid/internal/agent"
	"github.com/404cnf/meshgrid/internal/controller"
	"github.com/404cnf/meshgrid/internal/core"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "controller":
		runController(os.Args[2:])
	case "agent":
		runAgent(os.Args[2:])
	case "apply":
		apiCommand("POST", "/v1/workloads", os.Args[2:])
	case "get":
		runGet(os.Args[2:])
	case "delete":
		runDelete(os.Args[2:])
	case "identity":
		apiCommand("GET", "/v1/identity", os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}
func usage() { fmt.Fprintln(os.Stderr, "meshgrid controller|agent|apply|get|delete|identity") }
func runController(args []string) {
	fs := flag.NewFlagSet("controller", flag.ExitOnError)
	listen := fs.String("listen", ":7443", "listen address")
	data := fs.String("data", "./meshgrid-data", "state directory")
	token := fs.String("token", os.Getenv("MESHGRID_TOKEN"), "cluster token")
	_ = fs.Parse(args)
	if *token == "" {
		log.Fatal("--token or MESHGRID_TOKEN is required")
	}
	id, err := core.LoadOrCreateIdentity(filepath.Join(*data, "identity.json"))
	must(err)
	store, err := core.OpenStore(filepath.Join(*data, "state.json"))
	must(err)
	srv := &http.Server{Addr: *listen, Handler: (&controller.Server{Store: store, Identity: id, Token: *token}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("controller %s listening on %s", id.ID, *listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(c)
}
func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	controllerURL := fs.String("controller", "http://127.0.0.1:7443", "controller URL")
	token := fs.String("token", os.Getenv("MESHGRID_TOKEN"), "cluster token")
	id := fs.String("id", hostname(), "node id")
	name := fs.String("name", hostname(), "node name")
	cpu := fs.Int64("cpu", 4000, "CPU capacity in millicores")
	memory := fs.Int64("memory", 8192, "memory capacity in MB")
	gpus := fs.Int("gpus", 0, "GPU count")
	labels := fs.String("labels", "", "comma-separated key=value labels")
	federate := fs.Bool("allow-federation", false, "allow explicitly federated workloads")
	dry := fs.Bool("dry-run", false, "do not invoke Docker")
	_ = fs.Parse(args)
	if *token == "" {
		log.Fatal("--token or MESHGRID_TOKEN is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := agent.Config{Controller: *controllerURL, Token: *token, NodeID: *id, Name: *name, Labels: parseLabels(*labels), Capacity: core.Resources{CPUMillis: *cpu, MemoryMB: *memory, GPUs: *gpus}, AllowFederation: *federate, DryRun: *dry}
	must(agent.New(cfg).Run(ctx))
}
func runGet(args []string) {
	if len(args) == 0 {
		log.Fatal("get requires nodes, workloads, or status")
	}
	path := map[string]string{"nodes": "/v1/nodes", "workloads": "/v1/workloads", "status": "/v1/status"}[args[0]]
	if path == "" {
		log.Fatal("unknown resource")
	}
	apiCommand("GET", path, args[1:])
}
func runDelete(args []string) {
	if len(args) == 0 {
		log.Fatal("delete requires workload ID")
	}
	apiCommand("DELETE", "/v1/workloads/"+args[0], args[1:])
}
func apiCommand(method, path string, args []string) {
	fs := flag.NewFlagSet("api", flag.ExitOnError)
	controllerURL := fs.String("controller", "http://127.0.0.1:7443", "controller URL")
	token := fs.String("token", os.Getenv("MESHGRID_TOKEN"), "cluster token")
	file := fs.String("f", "", "JSON file")
	_ = fs.Parse(args)
	var body io.Reader
	if *file != "" {
		b, err := os.ReadFile(*file)
		must(err)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(*controllerURL, "/")+path, body)
	must(err)
	req.Header.Set("Authorization", "Bearer "+*token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	must(err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	must(err)
	var pretty bytes.Buffer
	if json.Indent(&pretty, b, "", "  ") == nil {
		b = pretty.Bytes()
	}
	fmt.Println(string(b))
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
}
func parseLabels(s string) map[string]string {
	m := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(p, "=")
		if ok && k != "" {
			m[k] = v
		}
	}
	return m
}
func hostname() string { h, _ := os.Hostname(); return h }
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
