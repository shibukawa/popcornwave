// Package authfaste2e drives the authentication endpoints over fasthttp,
// against a real identity provider and a real database.
//
// It is the fasthttp counterpart of passkeye2e, and it exists for the same
// reason that one does: almost everything worth proving about a login is
// decided by parts that only exist when they are wired together. A ceremony
// cookie has to survive a redirect the browser follows, a transaction has to be
// consumed exactly once, a rotated session has to replace the cookie the last
// one set, and a guard has to observe the authentication a frame two positions
// above it recorded. None of that is visible to a unit test holding one
// handler.
//
// What is new here, and the whole reason for a second e2e, is the transport.
// The decisions are plugin/auth's and are already covered; what is not covered
// anywhere else is that a fasthttp request value carries them — that a cookie
// set through the translated writer comes back, that a pooled request answers
// the session lookup, that a JSON body read after the handler was entered is
// the body that arrived.
//
// # One process, one runtime, two listeners
//
// The fixture builds the net/http chain first, because that is what performs
// framework startup today: configuration parsing, the database, the session
// manager. The fasthttp chain then reads the authentication runtime that
// startup installed rather than building a second one. Both listeners serve,
// so a test can ask the same question twice and compare the answers.
package authfaste2e
