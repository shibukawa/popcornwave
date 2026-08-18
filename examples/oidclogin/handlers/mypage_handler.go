package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pw"
)

func init() { mux.HandleFunc("GET /mypage", myPage) }

// myPage is listed in auth.protection.include, so the guard has already
// rejected or redirected every unauthenticated request before this runs. The
// handler still reads the verified authentication result rather than assuming
// a cookie is present.
func myPage(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	pw.WriteHTML(w, r, MyPage(MyPageParams{
		DisplayName: user.DisplayName,
		AccountID:   user.AccountID,
		Issuer:      user.Issuer,
		KeyClaim:    user.KeyClaim,
		Key:         user.Key,
		Subject:     user.Subject,
		Method:      pw.RequestAuthentication(r).Method,
	}))
}
