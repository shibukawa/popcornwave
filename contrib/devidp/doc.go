// Package devidp implements a development-only OpenID Provider.
//
// The provider authenticates by letting a developer select a user from a TOML
// roster; it never checks a credential. It implements the Authorization Code
// grant with mandatory S256 PKCE, RS256 ID Tokens published through a JWKS
// endpoint, discovery metadata, and UserInfo, which is the subset a Popcorn
// Wave relying party needs.
//
// The package is host-only tooling. It must never be imported by an
// application binary: pw build rejects a project that does. Everything it
// issues lives in memory and dies with the process.
package devidp
