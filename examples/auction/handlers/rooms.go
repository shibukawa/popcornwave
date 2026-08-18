package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"auction/internal/lobby"
	"auction/queries"
	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pw"
)

// maxBidDollars keeps a posted figure clear of the overflow that turning
// dollars into cents would otherwise reach.
const maxBidDollars = 1_000_000_000

type createRoomInput struct {
	Title              string `payload:"title" check:"required,maxlen=80"`
	Subject            string `payload:"subject" check:"required,maxlen=120"`
	SubjectDescription string `payload:"subject_description" check:"maxlen=1000"`
	StartingAmount     string `payload:"starting_amount" check:"required,pattern=^[0-9]+([.][0-9][0-9]?)?$"`
}

type joinRoomInput struct {
	RoomID int `payload:"room_id" check:"min=1"`
}

type auctionRoomPathInput struct {
	RoomID int `path:"roomID" check:"min=1"`
}

type bidInput struct {
	RoomID int    `path:"roomID" check:"min=1"`
	Amount string `payload:"amount" check:"required,pattern=^[0-9]+([.][0-9][0-9]?)?$"`
}

type hostMessageInput struct {
	RoomID  int    `path:"roomID" check:"min=1"`
	Message string `payload:"message" check:"required,maxlen=500"`
}

func init() {
	mux.HandleFunc("POST /rooms", createRoom)
	mux.HandleFunc("POST /rooms/join", joinRoom)
	mux.HandleFunc("POST /rooms/{roomID}/bid", placeBid)
	mux.HandleFunc("POST /rooms/{roomID}/message", postHostMessage)
	mux.HandleFunc("POST /rooms/{roomID}/close", closeRoom)
}

func createRoom(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[createRoomInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	startingAmount, err := parseAmountCents(input.StartingAmount)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if _, err := queries.UpsertAccount(r.Context(), user.AccountID, user.DisplayName, user.Email); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	room, err := queries.CreateRoom(
		r.Context(),
		user.AccountID,
		input.Title,
		input.Subject,
		input.SubjectDescription,
		startingAmount,
	)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	lobby.NotifyRoomsChanged()
	pw.RedirectSeeOther(w, r, "/rooms/"+strconv.Itoa(room.Id))
}

func joinRoom(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[joinRoomInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if _, err := queries.UpsertAccount(r.Context(), user.AccountID, user.DisplayName, user.Email); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if _, err := queries.JoinRoom(r.Context(), input.RoomID, user.AccountID); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	lobby.NotifyRoomsChanged()
	lobby.NotifyRoomChanged(input.RoomID)
	pw.RedirectSeeOther(w, r, "/rooms/"+strconv.Itoa(input.RoomID))
}

func placeBid(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[bidInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	amount, err := parseAmountCents(input.Amount)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	room, err := queries.GetAuctionRoom(r.Context(), input.RoomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if room == nil {
		pw.WriteProblem(w, r, pw.NotFound("no such auction room"))
		return
	}
	if room.Status != "open" {
		pw.WriteProblem(w, r, pw.Conflict("this auction is closed"))
		return
	}
	if !room.ViewerIsParticipant {
		pw.WriteProblem(w, r, pw.Forbidden("join the room before bidding"))
		return
	}
	if amount <= room.CurrentAmount {
		pw.WriteProblem(w, r, pw.Conflict("bid above the current amount"))
		return
	}
	if _, err := queries.PlaceBid(r.Context(), input.RoomID, user.AccountID, amount); err != nil {
		if isAuctionConflict(err) {
			pw.WriteProblem(w, r, pw.Conflict("bid rejected, check the current amount"))
		} else {
			pw.WriteProblem(w, r, err)
		}
		return
	}
	lobby.NotifyRoomChanged(input.RoomID)
	pw.RedirectSeeOther(w, r, "/rooms/"+strconv.Itoa(input.RoomID))
}

func postHostMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[hostMessageInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if strings.TrimSpace(input.Message) == "" {
		pw.WriteProblem(w, r, pw.BadRequest("comment must not be empty"))
		return
	}
	room, err := queries.GetAuctionRoom(r.Context(), input.RoomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if room == nil {
		pw.WriteProblem(w, r, pw.NotFound("no such auction room"))
		return
	}
	if !room.ViewerIsCreator {
		pw.WriteProblem(w, r, pw.Forbidden("only the host can comment"))
		return
	}
	if room.Status != "open" {
		pw.WriteProblem(w, r, pw.Conflict("this auction is closed"))
		return
	}
	if _, err := queries.PostHostMessage(r.Context(), input.RoomID, user.AccountID, input.Message); err != nil {
		if isAuctionConflict(err) {
			pw.WriteProblem(w, r, pw.Conflict("this auction is already closed"))
		} else {
			pw.WriteProblem(w, r, err)
		}
		return
	}
	lobby.NotifyRoomChanged(input.RoomID)
	pw.RedirectSeeOther(w, r, "/rooms/"+strconv.Itoa(input.RoomID))
}

func closeRoom(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[auctionRoomPathInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	room, err := queries.GetAuctionRoom(r.Context(), input.RoomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if room == nil {
		pw.WriteProblem(w, r, pw.NotFound("no such auction room"))
		return
	}
	if !room.ViewerIsCreator {
		pw.WriteProblem(w, r, pw.Forbidden("only the host can close the auction"))
		return
	}
	if room.Status != "open" {
		pw.WriteProblem(w, r, pw.Conflict("this auction is already closed"))
		return
	}
	result, err := queries.CloseAuctionRoom(r.Context(), input.RoomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if affected, err := result.RowsAffected(); err != nil {
		pw.WriteProblem(w, r, err)
		return
	} else if affected == 0 {
		pw.WriteProblem(w, r, pw.Conflict("this auction is already closed"))
		return
	}
	lobby.NotifyRoomChanged(input.RoomID)
	lobby.NotifyRoomsChanged()
	pw.RedirectSeeOther(w, r, "/rooms/"+strconv.Itoa(input.RoomID))
}

// parseAmountCents turns the dollar figure a form posted — "10" and "10.01"
// are both accepted — into the cents the database stores. The pattern check on
// the field has already settled the shape, so what is left to reject here is a
// figure too large to survive the multiplication.
func parseAmountCents(value string) (int, error) {
	whole, fraction, hasFraction := strings.Cut(value, ".")
	dollars, err := strconv.Atoi(whole)
	if err != nil || dollars > maxBidDollars {
		return 0, pw.BadRequest("amount must be at most " + strconv.Itoa(maxBidDollars) + " dollars")
	}
	if !hasFraction {
		return dollars * 100, nil
	}
	if len(fraction) == 1 {
		fraction += "0"
	}
	cents, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, pw.BadRequest("amount must be a dollar figure such as 10 or 10.01")
	}
	return dollars*100 + cents, nil
}

func isAuctionConflict(err error) bool {
	message := err.Error()
	return strings.Contains(message, "auction room is not open") ||
		strings.Contains(message, "bid must be greater") ||
		strings.Contains(message, "only the room host") ||
		strings.Contains(message, "only a room participant")
}
