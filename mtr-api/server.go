package main

import (
	"database/sql"
	_ "github.com/GeoNet/cfg/cfgenv"
	"github.com/GeoNet/map180"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"time"
)

var mux *http.ServeMux
var db *sql.DB
var dbR *sql.DB // Database connection with read only credentials
var userW, keyW string
var userR, keyR string
var wm *map180.Map180

var eol = []byte("\n")

func init() {
	userW = os.Getenv("MTR_USER")
	keyW = os.Getenv("MTR_KEY")
	userR = os.Getenv("MTR_USER_R")
	keyR = os.Getenv("MTR_KEY_R")

	mux = http.NewServeMux()
	mux.HandleFunc("/field/locality", toHandler(fieldLocalityHandler))
	mux.HandleFunc("/field/locality/dark", toHandler(fieldLocalityDarkHandler))
	mux.HandleFunc("/field/source", toHandler(fieldSourceHandler))
	mux.HandleFunc("/field/metric", toHandler(fieldMetricHandler))
	mux.HandleFunc("/field/metric/latest", toHandler(fieldMetricLatestHandler))
	mux.HandleFunc("/field/metric/threshold", toHandler(fieldThresholdHandler))
	mux.HandleFunc("/field/metric/tag", toHandler(fieldTagHandler))
	mux.HandleFunc("/field/metric/type", toHandler(fieldTypeHandler))
	mux.HandleFunc("/health", health)
}

func main() {
	var err error
	db, err = sql.Open("postgres",
		os.ExpandEnv("host=${DB_HOST} connect_timeout=30 user=${DB_USER} password=${DB_PASSWORD} dbname=mtr sslmode=disable"))
	if err != nil {
		log.Println("Problem with DB config.")
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(10)

	if err = db.Ping(); err != nil {
		log.Println("ERROR: problem pinging DB - is it up and contactable? 500s will be served")
	}

	dbR, err = sql.Open("postgres",
		os.ExpandEnv("host=${DB_HOST} connect_timeout=30 user=${DB_USER_R} password=${DB_PASSWORD_R} dbname=mtr sslmode=disable"))
	if err != nil {
		log.Println("Problem with DB config.")
		log.Fatal(err)
	}
	defer dbR.Close()

	dbR.SetMaxIdleConns(20)
	dbR.SetMaxOpenConns(20)

	if err = dbR.Ping(); err != nil {
		log.Println("ERROR: problem pinging DB - is it up and contactable? 500s will be served")
	}

	// For map zoom regions other than NZ will need to read some config from somewhere.
	wm, err = map180.Init(dbR, map180.Region(`newzealand`), 256000000)
	if err != nil {
		log.Println("ERROR: problem with map180 config: %s", err)
	}

	go deleteMetrics()

	log.Println("starting server")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

/*
health does not require auth - for use with AWS EB load balancer checks.
*/
func health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

/*
deleteMetics deletes old metrics.
*/
func deleteMetrics() {
	ticker := time.NewTicker(time.Minute * 1).C
	var err error

	for {
		select {
		case <-ticker:
			if _, err = db.Exec(`DELETE FROM field.metric_minute WHERE time < now() - interval '1440 minutes'`); err != nil {
				log.Println(err)
			}
			if _, err = db.Exec(`DELETE FROM field.metric_hour WHERE time < now() - interval '1440 hours'`); err != nil {
				log.Println(err)
			}
			if _, err = db.Exec(`DELETE FROM field.metric_day WHERE time < now() - interval '1440 days'`); err != nil {
				log.Println(err)
			}
		}
	}
}
