package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"testing"
)

var testServer *httptest.Server

func setup(t *testing.T) {
	var err error
	if db, err = sql.Open("postgres",
		os.ExpandEnv("host=${DB_HOST} connect_timeout=30 user=${DB_USER} password=${DB_PASSWORD} dbname=mtr sslmode=disable")); err != nil {
		t.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		t.Fatal(err)
	}

	dbR, err = sql.Open("postgres",
		os.ExpandEnv("host=${DB_HOST} connect_timeout=30 user=${DB_USER_R} password=${DB_PASSWORD_R} dbname=mtr sslmode=disable"))
	if err != nil {
		t.Fatal(err)
	}

	if err = dbR.Ping(); err != nil {
		t.Fatal(err)
	}

	testServer = httptest.NewServer(mux)
}

func teardown() {
	testServer.Close()
	db.Close()
	defer dbR.Close()
}

// loc returns a string representing the line of code 2 functions calls back.
func loc() (loc string) {
	_, _, l, _ := runtime.Caller(2)
	return "L" + strconv.Itoa(l)
}
