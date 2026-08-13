package handlers

// ClientMsg is everything a browser sends. The Type field is the protocol's
// discriminator; the library names nothing here, because the protocol is the
// application's.
//
// No field carries omitempty, which the generated encoder would honour: a
// tagged field that is empty would stop being sent. Every field is written
// instead, so a client reads the discriminator and ignores the ones that
// variant does not use, and never has an absent case to handle.
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
