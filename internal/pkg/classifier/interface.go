package classifier

import "context"

// The all possible results of DiscriminatorPlugin's AnalyzeHeader
const (
	// The discriminator confirmed the protocol of the tunnel's traffic and extra all properties of the protocol.
	AFFIRM = 0x00
	// The discriminator can't confirm the protocol of the tunnel's traffic, means need more data.
	UNCERTAINTY = 0x01
	// The procotol already confirmed, but need more data to fill properties
	INCOMPLETE = 0x02
	// The discriminator confirmed the protocol of the tunnel's traffic isn't the protocol of the discriminator
	DENY = 0x03
)

type DiscriminatorPlugin interface {
	// Analy the header data and make a determination whether or not the protocol
	// of the traffic is protocol corresponding to the discriminator. client is the
	// header which from client application, server is the header data whih from server
	// application.
	AnalyzeHeader(ctx context.Context, client *[]byte, server *[]byte) (result int)
	// If the protocol is hit, discriminator will try to extra properties
	// related the protocol from the header.
	GetProperties(ctx context.Context) (properties any)
}

// TODO(jeffyjf) Now, we just provide a spice discriminator, so we return it directly.
// If we going to add more discriminators in future, the method should be refactor in advance.
func LoadDiscriminators() map[string]DiscriminatorPlugin {
	return map[string]DiscriminatorPlugin{"spice": &spiceDiscriminator{}}
}
