package classifier

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/log"
)

const (
	SPICE_MAGIC         = "REDQ"
	MAJOR_VERSION_INDEX = 4
	MINOR_VERSION_INDEX = 8
	CHANNEL_TYPE_INDEX  = 20
	CHANNEL_MAIN        = 1
	CHANNEL_DISPLAY     = 2
	CHANNEL_INPUTS      = 3
	CHANNEL_CURSOR      = 4
	CHANNEL_PLAYBACK    = 5
	CHANNEL_RECORD      = 6
	CHANNEL_TUNNEL      = 7
	CHANNEL_SMARTCARD   = 8
	CHANNEL_USBREDIR    = 9
	CHANNEL_PORT        = 10
	CHANNEL_WEBDAV      = 11
)

const (
	INITIAL_OFFSET            = 12 // The index of the server link message's message size
	SPICE_LINK_ERR_OK         = 0
	MESSAGE_TYPE_INIT         = 103
	MESSAGE_TYPE_SERVER_NAME  = 113
	MESSAGE_TYPE_SERVER_UUID  = 114
	MESSAGE_SIZE_LENGTH       = 4
	MESSAGE_TYPE_LENGTH       = 2
	LINK_STATUS_LENGTH        = 4
	SESSION_ID_LENGTH         = 4
	SERVER_NAME_LENGTH_LENGTH = 4
)

type spiceProperties struct {
	Version     string `json:"version"`
	SessionId   string `json:"sessionId"`
	ChannelType string `json:"channelType"`
	ServerName  string `json:"serverName,omitempty"`
	ServerUUID  string `json:"serverUUID,omitempty"`
}

type spiceDiscriminator struct {
	properties spiceProperties
}

func (s *spiceDiscriminator) analyzeServerHeader(logger log.Logger, server *[]byte) int {
	logger.Info("Alalyze server application header data.")
	// Get the first paket message size
	var offset int = INITIAL_OFFSET + MESSAGE_SIZE_LENGTH
	if len(*server) < offset {
		return INCOMPLETE
	}
	messageSize := binary.LittleEndian.Uint32((*server)[offset-MESSAGE_SIZE_LENGTH : offset])
	// We cannot extract any property from the first packet, so skip it. Set the offset to link status
	offset = offset + int(messageSize) + LINK_STATUS_LENGTH
	if len(*server) < offset {
		return INCOMPLETE
	}
	linkStatus := binary.LittleEndian.Uint32((*server)[offset-LINK_STATUS_LENGTH : offset])
	if linkStatus != SPICE_LINK_ERR_OK {
		logger.Info("The link status of main channel isn't 'OK'")
		return AFFIRM
	}
	// From different type message extract different properties.
	var messageTypeMap map[string]int = map[string]int{
		"init":       MESSAGE_TYPE_INIT,
		"serverName": MESSAGE_TYPE_SERVER_NAME,
		"serverUUID": MESSAGE_TYPE_SERVER_UUID,
	}

	// Loop process the subsequent data, until we get all informations or encountered unknown packet.
	// Set a timer in order to avoid potential endless loop
	timer := time.NewTimer(10 * time.Second)
	for {
		select {
		case <-timer.C:
			logger.Errorw("The timer is timeout during spice discriminator analyze server application data for main channel.", "error", "Timeout")
			return AFFIRM
		default:
			offset = offset + MESSAGE_TYPE_LENGTH
			if len(*server) < offset {
				return INCOMPLETE
			}
			messageType := binary.LittleEndian.Uint16((*server)[offset-MESSAGE_TYPE_LENGTH : offset])
			switch messageType {
			case MESSAGE_TYPE_INIT:
				offset = offset + MESSAGE_SIZE_LENGTH
				if len(*server) < offset {
					return INCOMPLETE
				}
				messageSize := binary.LittleEndian.Uint32((*server)[offset-MESSAGE_SIZE_LENGTH : offset])
				offset = offset + int(messageSize)
				if len(*server) < offset {
					return INCOMPLETE
				}
				sessionIndex := offset - int(messageSize)
				s.properties.SessionId = fmt.Sprintf("%x", (*server)[sessionIndex:sessionIndex+SESSION_ID_LENGTH])
				delete(messageTypeMap, "init")
			case MESSAGE_TYPE_SERVER_NAME:
				offset = offset + MESSAGE_SIZE_LENGTH
				if len(*server) < offset {
					return INCOMPLETE
				}
				messageSize := binary.LittleEndian.Uint32((*server)[offset-MESSAGE_SIZE_LENGTH : offset])
				offset = offset + int(messageSize)
				if len(*server) < offset {
					return INCOMPLETE
				}
				nameLenIndex := offset - int(messageSize)
				nameLen := binary.LittleEndian.Uint32((*server)[nameLenIndex : nameLenIndex+SERVER_NAME_LENGTH_LENGTH])
				nameIndex := nameLenIndex + SERVER_NAME_LENGTH_LENGTH
				s.properties.ServerName = string((*server)[nameIndex : nameIndex+int(nameLen)-1])
				delete(messageTypeMap, "serverName")
			case MESSAGE_TYPE_SERVER_UUID:
				offset = offset + MESSAGE_SIZE_LENGTH
				if len(*server) < offset {
					return INCOMPLETE
				}
				messageSize := binary.LittleEndian.Uint32((*server)[offset-MESSAGE_SIZE_LENGTH : offset])
				offset = offset + int(messageSize)
				if len(*server) < offset {
					return INCOMPLETE
				}
				uuid, err := uuid.FromBytes((*server)[offset-int(messageSize) : offset])
				if err != nil {
					return AFFIRM
				}
				s.properties.ServerUUID = uuid.String()
				delete(messageTypeMap, "serverUUID")
			default:
				logger.Errorw("Encounter unkunown packet.", "error", "unknown")
				return AFFIRM
			}
			if len(messageTypeMap) == 0 {
				return AFFIRM
			}
		}
	}
}

// Refer docs: https://www.spice-space.org/spice-protocol.html
func (s *spiceDiscriminator) AnalyzeHeader(ctx context.Context, client *[]byte, server *[]byte) int {
	if len(*client) < 21 {
		return UNCERTAINTY
	}
	logger := log.FromContext(ctx)
	if string((*client)[:4]) != SPICE_MAGIC {
		return DENY
	}
	logger.Info("The protocol of the traffic that pass through the tunnel is spice.")
	// This means the properties haven't be instantiated (the first time that get enough
	// header data to analyzed the traffic's protocol is spice)
	if s.properties.ChannelType == "" {
		s.properties = spiceProperties{
			Version:   fmt.Sprintf("%x.%x", (*client)[MAJOR_VERSION_INDEX], (*client)[MINOR_VERSION_INDEX]),
			SessionId: fmt.Sprintf("%x", (*client)[16:20]),
		}
		// If the properties already instantiated, and the channel type is
		// main, we to analy the server header data directly.
	} else if s.properties.ChannelType == "main" {
		return s.analyzeServerHeader(logger, server)
	} else {
		return AFFIRM
	}
	switch (*client)[CHANNEL_TYPE_INDEX] {
	case CHANNEL_MAIN:
		s.properties.ChannelType = "main"
		// For main channel, we need analy server header data to extra more properties.
		logger.Info(fmt.Sprintf("The spice traffic's channel type is %s", s.properties.ChannelType))
		return s.analyzeServerHeader(logger, server)
	case CHANNEL_DISPLAY:
		s.properties.ChannelType = "display"
	case CHANNEL_INPUTS:
		s.properties.ChannelType = "inputs"
	case CHANNEL_CURSOR:
		s.properties.ChannelType = "cursor"
	case CHANNEL_PLAYBACK:
		s.properties.ChannelType = "playback"
	case CHANNEL_RECORD:
		s.properties.ChannelType = "record"
	case CHANNEL_TUNNEL:
		s.properties.ChannelType = "tunnel"
	case CHANNEL_SMARTCARD:
		s.properties.ChannelType = "smartcard"
	case CHANNEL_USBREDIR:
		s.properties.ChannelType = "usbredir"
	case CHANNEL_PORT:
		s.properties.ChannelType = "port"
	case CHANNEL_WEBDAV:
		s.properties.ChannelType = "webdev"
	default:
		s.properties.ChannelType = "unknow"
	}
	logger.Info(fmt.Sprintf("The spice traffic's channel type is %s", s.properties.ChannelType))
	return AFFIRM
}

func (s *spiceDiscriminator) GetProperties(ctx context.Context) any {
	return s.properties
}
