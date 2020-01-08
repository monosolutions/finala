package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"io/ioutil"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	log "github.com/sirupsen/logrus"

	"finala/serverutil"
	"finala/storage"
)

const (
	// DrainTimeout is how long to wait until the server is drained before closing it
	DrainTimeout = time.Second * 30	
)

var box = packr.NewBox("../ui/build");

// Server is the API server struct
type Server struct {
	router     *mux.Router
	httpserver *http.Server
	storage    storage.Storage
}

// NewServer returns a new Server
func NewServer(port int, storage storage.Storage) *Server {

	router := mux.NewRouter()
	corsObj := handlers.AllowedOrigins([]string{"*"})
	return &Server{
		router:  router,
		storage: storage,
		httpserver: &http.Server{
			Handler: handlers.CORS(corsObj)(router),
			Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		},
	}
}

// Serve starts the HTTP server and listens until StopFunc is called
func (server *Server) Serve() serverutil.StopFunc {
	ctx, cancelFn := context.WithCancel(context.Background())
	server.BindEndpoints()

	stopped := make(chan bool)
	go func() {
		<-ctx.Done()
		serverCtx, serverCancelFn := context.WithTimeout(context.Background(), DrainTimeout)
		err := server.httpserver.Shutdown(serverCtx)
		if err != nil {
			log.WithError(err).Error("error occured while shutting down manager HTTP server")
		}
		serverCancelFn()
		stopped <- true
	}()
	go func() {
		server.httpserver.ListenAndServe()
	}()
	return func() {
		cancelFn()
		<-stopped
		log.Warn("HTTP server has been drained and shut down")
	}
}

func interceptHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	next.ServeHTTP(&interceptResponseWriter{w, customErrorHandler}, r)
})
}

func customErrorHandler(w http.ResponseWriter, status int) {
	if status == 404 {
		file, _:= box.Open("/index.html")
		content, err := ioutil.ReadAll(file)
		if(err != nil) {
			panic("unreachable")
		}
		w.Header().Add("Content-Type", "text/html")
		w.WriteHeader(200)
		w.Write(content)
	}
}

// BindEndpoints sets up the router to handle API endpoints
func (server *Server) BindEndpoints() {

	
	server.router.PathPrefix("/static").Handler(http.StripPrefix("/static",interceptHandler(http.FileServer(box))))
	server.router.HandleFunc("/api/v1/summary", server.GetSummary).Methods("GET")               // HealthCheck
	server.router.HandleFunc("/api/v1/resources/{type}", server.GetResourceData).Methods("GET") // return list of job deployments
	server.router.HandleFunc("/api/v1/health", server.HealthCheckHandler).Methods("GET")        // HealthCheck

	server.router.NotFoundHandler = http.HandlerFunc(server.NotFoundRoute)

}

// Router returns the Gorilla Mux HTTP router defined for this server
func (server *Server) Router() *mux.Router {
	return server.router
}

// JSONWrite return JSON response to the client
func (server *Server) JSONWrite(resp http.ResponseWriter, statusCode int, data interface{}) error {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(statusCode)
	encoder := json.NewEncoder(resp)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
