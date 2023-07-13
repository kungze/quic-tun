package restfulapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/kungze/quic-tun/pkg/tunnel"
)

type errorResponse struct {
	Msg string `json:"message"`
}

type httpd struct {
	// The socket address of the API server listen on
	ListenAddr string
}

func (h *httpd) getAllStreams(w http.ResponseWriter, request *http.Request) {
	var resp_json []byte
	var err error
	if request.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		resp_json, _ = json.Marshal(errorResponse{Msg: "Please use GET request method"})
	} else {
		allTuns := tunnel.DataStore.LoadAll()
		resp_json, err = json.Marshal(allTuns)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp_json = []byte(err.Error())
		}
	}
	_, err = w.Write(resp_json)
	if err != nil {
		log.Errorw("Encounter error!", "error", err.Error())
	}
}

func (h *httpd) closeStream(w http.ResponseWriter, request *http.Request) {
	var resp_json []byte
	if request.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		resp_json, _ = json.Marshal(errorResponse{Msg: "Please use PUT request method"})
	} else {
		streamUuid, _ := uuid.Parse(mux.Vars(request)["uuid"])
		tun, err := tunnel.DataStore.LoadOne(streamUuid)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp_json, _ = json.Marshal(errorResponse{Msg: "Not found tunnel for uuid"})
		} else {
			(*tun.Stream).Close()
			(*tun.Conn).Close()
		}
	}
	_, err := w.Write(resp_json)
	if err != nil {
		log.Errorw("Encounter error!", "error", err.Error())
	}
}

func (h *httpd) Start() {
	router := mux.NewRouter()
	router.HandleFunc("/tunnels", h.getAllStreams)
	router.HandleFunc("/{uuid}/close_tunnel", h.closeStream)
	err := http.ListenAndServe(h.ListenAddr, router)
	if err != nil {
		panic(err)
	}
}

func NewHttpd(listenAddr string) httpd {
	return httpd{ListenAddr: listenAddr}
}
