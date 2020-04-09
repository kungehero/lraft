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
	//config
	{
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
	}
	
	//raft open
	{
		if err := s.Open(config.JoinAddr == "", config.NodeID); err != nil {
			log.Fatalf("failed to start HTTP service: %s", err.Error())
		}
	}
	
	//go-restful webservice
	{
		u := &lraft.LeveldbResource{Store: s}
		restful.DefaultContainer.Add(u.WebService())
	}
	
	//swagger ui
	{
		rfconfig := restfulspec.Config{
			WebServices:                   restful.RegisteredWebServices(),
			APIPath:                       "/apidocs.json",
			PostBuildSwaggerObjectHandler: enrichSwaggerObject}
		restful.DefaultContainer.Add(restfulspec.NewOpenAPIService(rfconfig))
		// Open http://localhost:8080/apidocs/?url=http://localhost:8080/apidocs.json
		http.Handle("/apidocs/", http.StripPrefix("/apidocs/", http.FileServer(http.Dir("/swagger-ui/dist"))))
	}

	//raft join
	{
		if config.JoinAddr != "" {
			if err := s.Join(config.NodeID, config.JoinAddr); err != nil {
				log.Fatalf("failed to join node at %s: %s", config.JoinAddr, err.Error())
			}
		}
	}

	//http start
	{
		log.Printf("start listening on:" + config.HttpAddr)
		fmt.Println("start successfuly!")
		if err := http.ListenAndServe(config.HttpAddr, nil); err != nil {
			fmt.Println(err)
			return
		}
	}
	
	//singal exit
	{
		terminate := make(chan os.Signal, 1)
		signal.Notify(terminate, os.Interrupt)
		<-terminate
		log.Println("hraftd exiting")
	}
}
