package session

// Placement states what the client may do with a registered slot and where its
// bytes live. It is declared where the type is registered, because what a
// client may do with a value is a property of the value rather than of the
// deployment. The deployment is left with one choice, which server backend a
// server-placed slot uses.
type Placement int

const (
	// Shared is a plain cookie the client reads and writes. A value the client
	// writes cannot live on the server, so this placement is a cookie by
	// definition. A decoded value is request input and is validated like a
	// query parameter.
	Shared Placement = iota + 1

	// ReadOnly is a signed cookie the client reads but cannot change. The
	// payload stays readable, so it carries no secret.
	ReadOnly

	// Private is sealed and unreadable by the client. It rides a sealed cookie
	// while the session is anonymous and moves to the configured server backend
	// at the login rotation, so a visitor who never logs in costs the server
	// nothing. Its anonymous phase is bounded by the browser cookie budget; a
	// value that can grow past it is declared ServerOnly instead.
	Private

	// ServerOnly is sealed and always server-placed, including while the
	// session is anonymous. The argument is revocation rather than
	// confidentiality: sealing already hides a value from the client, but a
	// cookie-placed record cannot be taken back. An anonymous write creates a
	// server record, which is what this placement asks for.
	ServerOnly
)

// String implements fmt.Stringer.
func (p Placement) String() string {
	switch p {
	case Shared:
		return "shared"
	case ReadOnly:
		return "read_only"
	case Private:
		return "private"
	case ServerOnly:
		return "server_only"
	default:
		return "unknown"
	}
}

// valid reports whether p names a placement.
func (p Placement) valid() bool { return p >= Shared && p <= ServerOnly }

// cookiePlaced reports whether the slot always lives in its own browser cookie.
// Private is not cookie-placed in this sense: it shares the session record,
// which is a cookie only while the session is anonymous.
func (p Placement) cookiePlaced() bool { return p == Shared || p == ReadOnly }

// mode is the cookie protection a cookie-placed slot carries.
func (p Placement) mode() CookieMode {
	if p == Shared {
		return CookiePlain
	}
	return CookieSigned
}

// needsKeyring reports whether a slot of this placement cannot be served
// without a keyring. Shared is the only placement that protects nothing.
func (p Placement) needsKeyring() bool { return p != Shared }
