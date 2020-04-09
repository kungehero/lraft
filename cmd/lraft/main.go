package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	lraft "kungehero/lraft"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/oklog/run"
)

// Command line defaults
const (
	DefaultHTTPAddr = ":8087"
	DefaultRaftAddr = ":12001"
)

var (
	config     Config
	ErrNotRaft = "No Raft storage directory specified\n"
)

type Config struct {
	UseMem      bool
	HttpAddr    string
	RaftAddr    string
	RaftDir     string
	JoinAddr    string
	NodeID      string
	BloomFilter bool
	Count       int
}

func init() {
	fs := flag.NewFlagSet("user-group", flag.ExitOnError)
	fs.BoolVar(&config.UseMem, "usemem", false, "use in-memory storage for raft")
	fs.StringVar(&config.HttpAddr, "ha", DefaultHTTPAddr, "Set the http bind address")
	fs.StringVar(&config.RaftAddr, "ra", DefaultRaftAddr, "Set Raft bind address")
	fs.StringVar(&config.RaftDir, "v", "raft", "raft-data-path")
	fs.StringVar(&config.JoinAddr, "join", "", "Set join address, if any")
	fs.StringVar(&config.NodeID, "id", "node1", "node id")
	fs.BoolVar(&config.BloomFilter, "bf", false, "use bloomfilter for leveldb")
	fs.IntVar(&config.Count, "count", 3, "bloom count for leveldb")
	fs.Parse(os.Args[1:])

	/* flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path> \n", os.Args[0])
		flag.PrintDefaults()
	} */
}

func main() {
	//	flag.Parse()

	/* if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, ErrNotRaft)
		os.Exit(1)
	}

	// Ensure Raft storage exists.
	raftDir := flag.Arg(0) */
	if config.RaftDir == "" {
		fmt.Fprintf(os.Stderr, ErrNotRaft)
		os.Exit(1)
	}
	os.MkdirAll(config.RaftDir, 0700)
	s := &lraft.Store{}
	s.RaftDir = config.RaftDir
	s.RaftBind = config.RaftAddr
	s.UseMem = config.UseMem
	s.BloomFilter = config.BloomFilter
	s.Count = config.Count
	if err := s.Open(config.JoinAddr == "", config.NodeID); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}
	u := &lraft.LeveldbResource{Store: s}
	restful.DefaultContainer.Add(u.WebService())

	rfconfig := restfulspec.Config{
		WebServices:                   restful.RegisteredWebServices(),
		APIPath:                       "/apidocs.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject}
	restful.DefaultContainer.Add(restfulspec.NewOpenAPIService(rfconfig))

	// Optionally, you can install the Swagger Service which provides a nice Web UI on your REST API
	// You need to download the Swagger HTML5 assets and change the FilePath location in the config below.
	// Open http://localhost:8080/apidocs/?url=http://localhost:8080/apidocs.json
	http.Handle("/apidocs/", http.StripPrefix("/apidocs/", http.FileServer(http.Dir("/swagger-ui/dist"))))
	log.Printf("start listening on:" + config.HttpAddr)
	fmt.Println("start successfuly!")

	var g run.Group
	// start http server.
	{
		server := &http.Server{Addr: config.HttpAddr, Handler: http.DefaultServeMux}
		g.Add(func() error {
			return server.ListenAndServe()
		}, func(err error) {
			if err == http.ErrServerClosed {
				log.Println("internal server closed unexpectedly")
				return
			}
			if err := server.Shutdown(context.TODO()); err != nil {
				log.Println("shutdown server", err)
			}
		})
	}

	if config.JoinAddr != "" {
		if err := s.Join(config.NodeID, config.JoinAddr); err != nil {
			log.Fatalf("failed to join node at %s: %s", config.JoinAddr, err.Error())
		}
	}

	// termination by signal Interrupt
	{
		terminate := make(chan os.Signal, 1)
		g.Add(func() error {
			signal.Notify(terminate, os.Interrupt)
			<-terminate
			return nil
		}, func(error) {
			close(terminate)
		})
	}

	if err := g.Run(); err != nil {
		log.Println("err", err)
	}

	log.Println("lraftd exiting")
}
