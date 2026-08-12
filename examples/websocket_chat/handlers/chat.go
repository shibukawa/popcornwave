package handlers

// ClientMsg is everything a browser sends. The Type field is the protocol's
// discriminator; the library names nothing here, because the protocol is the
// application's.
//
// No field carries omitempty. The generated encoder writes every field of the
// struct, so the tag would say something the wire does not do — a client reads
// the discriminator and ignores the fields that variant does not use.
type ClientMsg struct {
	Type string `json:"type"` // "join" | "say"
	Name string `json:"name"`
	Text string `json:"text"`
}

// ServerMsg is everything this room answers with.
type ServerMsg struct {
	Type string `json:"type"` // "welcome" | "presence" | "message" | "error"
	From string `json:"from"`
	Text string `json:"text"`
	Live int    `json:"live"`
	Code string `json:"code"`
}
