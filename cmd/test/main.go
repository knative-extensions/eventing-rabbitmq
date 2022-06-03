package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
)

func handler(w http.ResponseWriter, req *http.Request) {
	dumpReq, _ := httputil.DumpRequest(req, true)
	log.Println(string(dumpReq) + " modified " + http.StatusText(http.StatusInternalServerError))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	n, err := w.Write([]byte(fmt.Sprint("yellow")))
	log.Println(n, err)
	//http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
