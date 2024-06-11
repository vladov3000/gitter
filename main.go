package main

import (
	"github.com/eatonphil/gosqlite"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

type Server struct {
	index   *template.Template
	counter atomic.Int64
}

type Post struct {
	Content string
	Created int64
}

func makeServer() (*Server, error) {
	index, error := template.ParseFiles("index.html")
	if error != nil {
		return nil, error
	}
	
	counter := atomic.Int64{}

	return &Server{index, counter}, nil
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

	listPosts, error := connection.Prepare(`SELECT content, created FROM posts ORDER BY created DESC LIMIT 10`)
	if error != nil {
		log.Print("Failed to prepare statement: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	error = listPosts.Exec()
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

	insertPost, error := connection.Prepare(`INSERT INTO posts (id, content) VALUES (?, ?)`)
	if error != nil {
		log.Print("Failed to prepare statement: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}
	defer insertPost.Close()

	id := rand.Int63()
	error = insertPost.Exec(id, post[0])
	if error != nil {
		log.Print("Failed to insert post: ", error)
		http.Error(writer, "Server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	server, error := makeServer()
	if error != nil {
		log.Fatal(error)
	}

	http.HandleFunc("/", server.getIndex)
	http.HandleFunc("/post", server.submitPost)
	http.Handle("/static/", http.FileServer(http.Dir(".")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
