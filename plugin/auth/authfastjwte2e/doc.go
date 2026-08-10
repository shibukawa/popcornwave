// Package authfastjwte2e drives auth.mode = "jwt_only" over fasthttp.
//
// It is a separate binary from authfaste2e because framework configuration is
// parsed once per process and a mode is a setting: one process serves one mode,
// so proving the third one takes a second process.
//
// jwt_only shares almost nothing with the ceremony modes. There is no login to
// begin, no callback to correlate, no session to establish, and no cookie of any
// kind — every request carries its own credential and is authenticated from
// scratch. What that leaves for a transport to get wrong is the request header
// it reads the credential from, the challenge headers a refusal must carry, and
// the authentication a frame records for the guard two positions below it.
package authfastjwte2e
