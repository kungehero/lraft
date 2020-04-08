package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	lraft "kungehero/lraft"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
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
	JoinAddr    string
	NodeID      string
	BloomFilter bool
	Count       int
}

func init() {
	flag.BoolVar(&config.UseMem, "usemem", false, "use in-memory storage for raft")
	flag.StringVar(&config.HttpAddr, "ha", DefaultHTTPAddr, "Set the http bind address")
	flag.StringVar(&config.RaftAddr, "ra", DefaultRaftAddr, "Set Raft bind address")
	flag.StringVar(&config.JoinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&config.NodeID, "id", "", "node id")
	flag.BoolVar(&config.BloomFilter, "bf", false, "use bloomfilter for leveldb")
	flag.IntVar(&config.Count, "count", 3, "bloom count for leveldb")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path> \n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, ErrNotRaft)
		os.Exit(1)
	}

	// Ensure Raft storage exists.
	raftDir := flag.Arg(0)
	if raftDir == "" {
		fmt.Fprintf(os.Stderr, ErrNotRaft)
		os.Exit(1)
	}
	os.MkdirAll(raftDir, 0700)
	s := &lraft.Store{}
	s.RaftDir = raftDir
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
	log.Printf("start listening on localhost:8088")
	fmt.Println("start successfuly!")

	if err := http.ListenAndServe(config.HttpAddr, nil); err != nil {
		fmt.Println(err)
		return
	}
	if config.JoinAddr != "" {
		if err := s.Join(config.NodeID, config.JoinAddr); err != nil {
			log.Fatalf("failed to join node at %s: %s", config.JoinAddr, err.Error())
		}
	}

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	log.Println("hraftd exiting")

}
