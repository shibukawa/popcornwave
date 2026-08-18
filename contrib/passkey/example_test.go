package passkey_test

import (
	"fmt"

	"github.com/shibukawa/popcornweb/contrib/passkey"
)

func ExampleNew() {
	rp, err := passkey.New(passkey.Config{
		RPID: "example.com", RPName: "Example",
		Origins: []string{"https://example.com"},
	})
	if err != nil {
		panic(err)
	}
	request, _, err := rp.BeginAuthentication(nil, passkey.AuthenticationOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println(request.RPID)
	// Output: example.com
}
