package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"auction/internal/lobby"
	"auction/queries"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
)

type createRoomInput struct {
	Title              string `payload:"title" check:"required,maxlen=80"`
	Subject            string `payload:"subject" check:"required,maxlen=120"`
	SubjectDescription string `payload:"subject_description" check:"maxlen=1000"`
	StartingAmount     int    `payload:"starting_amount" check:"min=0"`
}

type joinRoomInput struct {
	RoomID int `payload:"room_id" check:"min=1"`
}

type auctionRoomPathInput struct {
	RoomID int `path:"roomID" check:"min=1"`
}

type bidInput struct {
	RoomID int `path:"roomID" check:"min=1"`
	Amount int `payload:"amount" check:"required,min=1"`
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
		input.StartingAmount,
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
	room, err := queries.GetAuctionRoom(r.Context(), input.RoomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if room == nil {
		pw.WriteProblem(w, r, pw.NotFound("オークションルームが見つかりません"))
		return
	}
	if room.Status != "open" {
		pw.WriteProblem(w, r, pw.Conflict("終了したルームには入札できません"))
		return
	}
	if !room.ViewerIsParticipant {
		pw.WriteProblem(w, r, pw.Forbidden("このルームに参加していません"))
		return
	}
	if input.Amount <= room.CurrentAmount {
		pw.WriteProblem(w, r, pw.Conflict("現在の金額より大きい金額を入力してください"))
		return
	}
	if _, err := queries.PlaceBid(r.Context(), input.RoomID, user.AccountID, input.Amount); err != nil {
		if isAuctionConflict(err) {
			pw.WriteProblem(w, r, pw.Conflict("入札できません。最新の金額を確認してください"))
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
		pw.WriteProblem(w, r, pw.BadRequest("コメントを入力してください"))
		return
	}
	room, err := queries.GetAuctionRoom(r.Context(), input.RoomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if room == nil {
		pw.WriteProblem(w, r, pw.NotFound("オークションルームが見つかりません"))
		return
	}
	if !room.ViewerIsCreator {
		pw.WriteProblem(w, r, pw.Forbidden("主催者だけがコメントできます"))
		return
	}
	if room.Status != "open" {
		pw.WriteProblem(w, r, pw.Conflict("終了したルームにはコメントできません"))
		return
	}
	if _, err := queries.PostHostMessage(r.Context(), input.RoomID, user.AccountID, input.Message); err != nil {
		if isAuctionConflict(err) {
			pw.WriteProblem(w, r, pw.Conflict("このルームはすでに終了しています"))
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
		pw.WriteProblem(w, r, pw.NotFound("オークションルームが見つかりません"))
		return
	}
	if !room.ViewerIsCreator {
		pw.WriteProblem(w, r, pw.Forbidden("主催者だけが終了を宣言できます"))
		return
	}
	if room.Status != "open" {
		pw.WriteProblem(w, r, pw.Conflict("このルームはすでに終了しています"))
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
		pw.WriteProblem(w, r, pw.Conflict("このルームはすでに終了しています"))
		return
	}
	lobby.NotifyRoomChanged(input.RoomID)
	lobby.NotifyRoomsChanged()
	pw.RedirectSeeOther(w, r, "/rooms/"+strconv.Itoa(input.RoomID))
}

func isAuctionConflict(err error) bool {
	message := err.Error()
	return strings.Contains(message, "auction room is not open") ||
		strings.Contains(message, "bid must be greater") ||
		strings.Contains(message, "only the room host") ||
		strings.Contains(message, "only a room participant")
}
