package main

import (
	"bytes"
	"encoding/binary"
	"github.com/eatonphil/gosqlite"
	"golang.org/x/crypto/argon2"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type Server struct {
	index    *template.Template
	counter  atomic.Int64
	sessions map[string]Session
}

type Post struct {
	Content string
	Created int64
}

type Session struct {
}

func makeServer() (*Server, error) {
	index, error := template.ParseFiles("index.html")
	if error != nil {
		return nil, error
	}

	connection, error := gosqlite.Open("gitter.db")
	if error != nil {
		return nil, error
	}
	defer connection.Close()

	getMaxPage, error := connection.Prepare(`SELECT MAX(page) FROM posts`)
	if error != nil {
		return nil, error
	}
	defer getMaxPage.Close()

	error = getMaxPage.Exec()
	if error != nil {
		return nil, error
	}

	existsMaxPage, error := getMaxPage.Step()
	if error != nil {
		return nil, error
	}

	counter := atomic.Int64{}
	if existsMaxPage {
		var page int64
		error = getMaxPage.Scan(&page)
		if error != nil {
			return nil, error
		}
		log.Printf("Found max page: %d", page)
		counter.Add(page)
	} else {
		log.Print("Did not find max page.")
	}

	return &Server{index, counter, nil}, nil
}

func (server *Server) signUp(writer http.ResponseWriter, request *http.Request) {
	log.Print("Signing up.")
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	error := request.ParseForm()
	if error != nil {
		http.Error(writer, "Malformed form parameters", http.StatusBadRequest)
		return
	}

	form := request.PostForm
	username := form.Get("username")
	password := form.Get("password")
	if username == "" || password == "" {
		http.Error(writer, "Missing message form parameter", http.StatusBadRequest)
		return
	}

	id := rand.Int63()

	salt := make([]byte, 8)
	binary.LittleEndian.PutUint64(salt, uint64(id))

	hashed_password := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	connection, error := gosqlite.Open("gitter.db")
	if error != nil {
		log.Print("Failed to open database connection: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	connection.BusyTimeout(5 * time.Second)
	defer connection.Close()

	insertUser, error := connection.Prepare(`INSERT INTO users VALUES (?, ?, ?)`)
	if error != nil {
		log.Print("Failed to prepare statement: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	defer insertUser.Close()

	error = insertUser.Exec(id, username, hashed_password)
	if error != nil {
		log.Print("Failed to insert user: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func (server *Server) login(writer http.ResponseWriter, request *http.Request) {
	log.Print("Logging in.")
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	error := request.ParseForm()
	if error != nil {
		http.Error(writer, "Malformed form parameters", http.StatusBadRequest)
		return
	}

	form := request.PostForm
	username := form.Get("username")
	password := form.Get("password")
	if username == "" || password == "" {
		http.Error(writer, "Missing message form parameter", http.StatusBadRequest)
		return
	}

	connection, error := gosqlite.Open("gitter.db")
	if error != nil {
		log.Print("Failed to open database connection: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	connection.BusyTimeout(5 * time.Second)
	defer connection.Close()

	selectUser, error := connection.Prepare(`SELECT id, hashed_password FROM users WHERE username = ?`)
	if error != nil {
		log.Print("Failed to prepare statement: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	defer selectUser.Close()

	error = selectUser.Exec(username)
	if error != nil {
		log.Print("Failed to find user: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	hasRow, error := selectUser.Step()
	if error != nil {
		log.Print("Failed to find user: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	if !hasRow {
		http.Error(writer, "Invalid username", http.StatusUnauthorized)
		return
	}

	var id int64
	var saved_hashed_password []byte
	error = selectUser.Scan(&id, &saved_hashed_password)
	if error != nil {
		log.Print("Failed to select user: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	salt := make([]byte, 8)
	binary.LittleEndian.PutUint64(salt, uint64(id))

	hashed_password := argon2.Key([]byte(password), salt, 1, 64*1024, 4, 32)
	if bytes.Equal(hashed_password, saved_hashed_password) {
		http.Error(writer, "Invalid password", http.StatusUnauthorized)
		return
	}

	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func (server *Server) getIndex(writer http.ResponseWriter, request *http.Request) {
	log.Print("Getting index.")
	if request.Method != http.MethodGet {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	connection, error := gosqlite.Open("gitter.db")
	if error != nil {
		log.Print("Failed to open database connection: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	connection.BusyTimeout(5 * time.Second)
	defer connection.Close()

	listPosts, error := connection.Prepare(`SELECT content, created FROM posts WHERE page <= ? ORDER BY created DESC LIMIT 10`)
	if error != nil {
		log.Print("Failed to prepare statement: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	defer listPosts.Close()

	parameters := request.URL.Query()
	pageParameters, _ := parameters["page"]

	var pageParameter int
	if pageParameters != nil {
		pageParameter, _ = strconv.Atoi(pageParameters[0])
	}

	postsPerPage := 10
	page := server.counter.Load()
	page -= int64(pageParameter * postsPerPage)

	error = listPosts.Exec(page)
	if error != nil {
		log.Print("Failed to list posts: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	var posts []Post
	for {
		hasRow, error := listPosts.Step()
		if error != nil {
			log.Print("Failed to list posts: ", error)
			http.Error(writer, "Server error", http.StatusInternalServerError)
			return
		}

		if !hasRow {
			break
		}

		var content string
		var created int64
		error = listPosts.Scan(&content, &created)
		if error != nil {
			log.Print("Failed to list posts: ", error)
			http.Error(writer, "Server error", http.StatusInternalServerError)
			return
		}

		posts = append(posts, Post{content, created})
	}

	index := server.index
	error = index.Execute(writer, posts)
	if error != nil {
		log.Print("Error while executing template: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
}

func (server *Server) submitPost(writer http.ResponseWriter, request *http.Request) {
	log.Print("Submitting post.")
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	error := request.ParseForm()
	if error != nil {
		http.Error(writer, "Malformed form parameters", http.StatusBadRequest)
		return
	}

	form := request.PostForm
	post, ok := form["message"]
	if !ok {
		http.Error(writer, "Missing message form parameter", http.StatusBadRequest)
		return
	}

	connection, error := gosqlite.Open("gitter.db")
	if error != nil {
		log.Print("Failed to open database connection: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	connection.BusyTimeout(5 * time.Second)
	defer connection.Close()

	insertPost, error := connection.Prepare(`INSERT INTO posts (id, page, content) VALUES (?, ?, ?)`)
	if error != nil {
		log.Print("Failed to prepare statement: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	defer insertPost.Close()

	id := rand.Int63()
	page := server.counter.Add(1)
	error = insertPost.Exec(id, page, post[0])
	if error != nil {
		log.Print("Failed to insert post: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func serveFile(name string) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, name);
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	server, error := makeServer()
	if error != nil {
		log.Fatal(error)
	}

	http.HandleFunc("/api/post", server.submitPost)
	http.HandleFunc("/api/signUp", server.signUp)
	http.HandleFunc("/api/login", server.login)
	
	http.HandleFunc("/", server.getIndex)
	http.HandleFunc("/login", serveFile("static/login.html"))
	http.HandleFunc("/signup", serveFile("static/signup.html"))
	http.Handle("/static/", http.FileServer(http.Dir(".")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
