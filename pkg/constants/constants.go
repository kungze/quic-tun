package constants

type contextkey string

const (
	// The length of token that client endpoint send to server endpoint
	TokenLength = 512
	// The lenght of ack message that server endpoint send to client endpoint
	AckMsgLength = 1
)

const (
	// Means that server endpoint accept the token which receive from client endpoint
	HandshakeSuccess = 0x01
	// Means that server endpoint cannot parse token
	ParseTokenError = 0x02
	// Means that server endpoint cannot connect server application
	CannotConnServer = 0x03
)

// The key name of klog's additional key/value pairs
const (
	ClientAppAddr      = "Client-App-Addr"
	StreamID           = "Stream-ID"
	ServerAppAddr      = "Server-App-Addr"
	ClientEndpointAddr = "Client-Endpoint-Addr"
)

// The value key names of value context
const (
	CtxClientAppAddrKey contextkey = "Client-App-Addr"
)

// The actions about tunnel data (which used to determine how to update tunnel data store in api server)
const (
	Creation = "creation"
	Close    = "close"
)
