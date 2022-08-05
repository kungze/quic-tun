package restfulapi

import (
	"encoding/json"
	"net/http"

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

func (h *httpd) Start() {
	http.HandleFunc("/tunnels", h.getAllStreams)
	err := http.ListenAndServe(h.ListenAddr, nil)
	if err != nil {
		panic(err)
	}
}

func NewHttpd(listenAddr string) httpd {
	return httpd{ListenAddr: listenAddr}
}
