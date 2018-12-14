package main

import (
	"github.com/codegangsta/negroni"
	"log"
	"net/http"
	_ "os"
)

func main() {
	initRSAKeys()
	port := "8080" //os.Getenv("PORT")

	/*if port == "" {
		log.Fatal("$PORT must be set")
	}*/

	// routes
	http.Handle("/", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.Wrap(http.StripPrefix("", http.FileServer(http.Dir("./statics")))),
	))
	http.Handle("/api/user/login", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.Wrap(http.HandlerFunc(loginHandler)),
	))
	http.Handle("/api/user/register", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.Wrap(http.HandlerFunc(registerHandler)),
	))
	http.Handle("/api/user/password", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(changePassword)),
	))
	http.Handle("/api/user/delete", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(deleteUser)),
	))
	http.Handle("/api/entry/list", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(listHandler)),
	))
	http.Handle("/api/entry/post", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(postHandler)),
	))
	http.Handle("/api/entry/view", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(entryHandler)),
	))
	http.Handle("/api/entry/decrypt", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(decryptHandler)),
	))
	http.Handle("/api/entry/delete", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(deleteHandler)),
	))
	http.Handle("/api/entry/edit", negroni.New(
		negroni.HandlerFunc(directToHttps),
		negroni.HandlerFunc(ValidateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(editHandler)),
	))

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
