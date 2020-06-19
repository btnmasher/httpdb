package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/btnmasher/lumberjack"
	"github.com/btnmasher/smallcfg"
	"github.com/gorilla/mux"
	"github.com/kr/pretty"
)

var (
	configFile string
	logger     *lumberjack.Logger

	Config struct {
		App AppSettings `json:"app"`
	}

	done     chan struct{}
	data     DataStore
	locks    LockStore
	showconf *bool
)

type AppSettings struct {
	Port         int           `json:"port"`
	Debug        bool          `json:"debug"`
	TimeOut      time.Duration `json:"timeout"`
	AtomicBuffer int           `json:"atomic_buffer"`
}

func init() {
	flag.StringVar(&configFile, "config", "httpdb.conf.json", "specify a config file")

	showconf = flag.Bool("showconf", false, "prints the configuration and exists.")

	flag.Parse()

	logger = lumberjack.NewLoggerWithDefaults()

	done = make(chan struct{})
	data = DataStore{Entries: make(map[string]*Entry)}
	locks = LockStore{Locks: make(map[string]struct{})}
}

func main() {

	logger.Info("======== Application Start =======")

	loadConfig()

	if *showconf {
		showConfig()
	}

	logger.Info("Starting Goroutines.")
	go startServer()
	go startAtomics(done)
	go startLockMinder(done)

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	sig := <-signalChannel
	switch sig {
	case os.Interrupt:
		logger.Info("SIGINT Received, shutting down...")
		close(done)
	case syscall.SIGTERM:
		logger.Info("SIGTERM Received, shutting down...")
		close(done)
	}

	time.Sleep(time.Second * 1)

	logger.Info("======== Application Exit ========")

}

func loadConfig() {

	smallcfg.Load(configFile, &Config)

	if Config.App.Debug {
		logger.AddLevel(lumberjack.DEBUG)
	}

	if Config.App.Port <= 0 || Config.App.Port >= 65534 {
		Config.App.Port = 9000
		logger.Warn("Service Port invalid or not specified in config, defaulting to 9000.")
	}

	if Config.App.TimeOut == 0 {
		Config.App.TimeOut = 5
	}

	if Config.App.AtomicBuffer < 1 {
		Config.App.AtomicBuffer = 1
		logger.Warn("Atomic buffer invalid or not specified in config, defaulting to 1.")
	}
}

func showConfig() {
	logger.Info("====== HttpDB Configuration ======")
	fmt.Println("---- App Settings:")
	pretty.Println(Config.App)
	logger.Info("==================================")
	logger.Info("Exiting application.")
	os.Exit(0)
}

func regHandlers(r *mux.Router) {
	logger.Info("Registering http handler routes...")

	r.HandleFunc("/reservations/{key}", reserveKey).Methods("POST")
	r.HandleFunc("/values/{key}", putVal).Methods("PUT")
	r.HandleFunc("/values/{key}/{lock_id}", updateVal).Methods("POST")
}

func startServer() {

	logger.Info("Started HTTP Server Gouroutine.")
	defer close(done)

	r := mux.NewRouter()
	regHandlers(r)
	http.Handle("/", r)

	logger.Infof("Listening for http connections on port %v!", Config.App.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", Config.App.Port), nil)

	if err != nil {
		logger.Error("ListenAndServe: ", err)
	}

	logger.Info("Listener stopped!")
}
