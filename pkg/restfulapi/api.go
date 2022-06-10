package restfulapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type Tunnel struct {
	Action             string        `json:"-"`
	Uuid               uuid.UUID     `json:"uuid"`
	StreamID           quic.StreamID `json:"streamId"`
	ClientAppAddr      string        `json:"clientAppAddr"`
	ServerAppAddr      string        `json:"serverAppAddr"`
	RemoteEndpointAddr string        `json:"remoteEndpointAddr"`
	CreatedAt          string        `json:"createdAt"`
}

type errorResponse struct {
	Msg string `json:"message"`
}

type httpd struct {
	// The socket address of the API server listen on
	ListenAddr string
	// Record all of the active tunnels
	DataStore map[uuid.UUID]Tunnel
	// Used to recevice tullen status info from client/server endpoint server
	Ch <-chan Tunnel
}

func (h *httpd) getAllStreams(w http.ResponseWriter, request *http.Request) {
	var resp_json []byte
	var err error
	if request.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		resp_json, _ = json.Marshal(errorResponse{Msg: "Please use GET request method"})
	} else {
		var allTuns []Tunnel
		for _, v := range h.DataStore {
			allTuns = append(allTuns, v)
		}
		resp_json, err = json.Marshal(allTuns)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp_json = []byte(err.Error())
		}
	}
	_, err = w.Write(resp_json)
	if err != nil {
		klog.ErrorS(err, "Encounter error!")
	}
}

func (h *httpd) Start() {
	go func() {
		for {
			data := <-h.Ch
			if data.Action == constants.Creation {
				h.DataStore[data.Uuid] = data
			}
			if data.Action == constants.Close {
				delete(h.DataStore, data.Uuid)
			}
		}
	}()
	http.HandleFunc("/tunnels", h.getAllStreams)
	err := http.ListenAndServe(h.ListenAddr, nil)
	if err != nil {
		panic(err)
	}
}

func NewHttpd(listenAddr string) (httpd, chan<- Tunnel) {
	dataChan := make(chan Tunnel, 100)
	return httpd{ListenAddr: listenAddr, Ch: dataChan, DataStore: map[uuid.UUID]Tunnel{}}, dataChan
}
