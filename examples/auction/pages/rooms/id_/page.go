package id_

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"auction/internal/lobby"
	"auction/queries"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
)

func Load(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	route, err := DecodeRoute(r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	roomID, err := strconv.Atoi(route.ID)
	if err != nil || roomID < 1 {
		pw.WriteProblem(w, r, pw.BadRequest("room id must be a positive number"))
		return
	}
	room, err := queries.GetAuctionRoom(r.Context(), roomID, user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if room == nil {
		pw.WriteProblem(w, r, pw.NotFound("no such auction room"))
		return
	}
	params := PageParams{
		Room:            toAuctionRoom(*room),
		ViewerAccountID: user.AccountID,
		DisplayName:     user.DisplayName,
		Email:           user.Email,
		BidPath:         BidPath(roomID),
		MessagePath:     MessagePath(roomID),
		ClosePath:       ClosePath(roomID),
		Live:            pw.WantsLive(r),
	}
	pw.WriteHTML(w, r, Page(params))
}

func WatchAuction(ctx context.Context, roomID int, viewerAccountID string) iter.Seq2[AuctionSnapshot, error] {
	return func(yield func(AuctionSnapshot, error) bool) {
		changed, unsubscribe := lobby.SubscribeRoom(roomID)
		defer unsubscribe()

		for {
			room, err := queries.GetAuctionRoom(ctx, roomID, viewerAccountID)
			if err != nil {
				yield(AuctionSnapshot{}, err)
				return
			}
			if room == nil {
				yield(AuctionSnapshot{}, pw.NotFound("no such auction room"))
				return
			}
			events, err := loadEvents(ctx, roomID)
			if err != nil {
				yield(AuctionSnapshot{}, err)
				return
			}
			snapshot := AuctionSnapshot{
				Room:      toAuctionRoom(*room),
				Events:    events,
				HasEvents: len(events) > 0,
			}
			if !yield(snapshot, nil) {
				return
			}
			if room.Status == "closed" {
				// The delivery above paints the final activity first. The signal then
				// asks screens that originally rendered the open controls to reload,
				// so those forms are replaced by the server-rendered auction result.
				yield(AuctionSnapshot{}, pw.NamedSignal("auction.closed"))
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-changed:
			}
		}
	}
}

func loadEvents(ctx context.Context, roomID int) ([]AuctionEvent, error) {
	events := make([]AuctionEvent, 0)
	for event, err := range queries.ListAuctionHistory(ctx, roomID) {
		if err != nil {
			return nil, err
		}
		events = append(events, AuctionEvent{
			Id:         event.Id,
			EventType:  event.EventType,
			Amount:     event.Amount,
			AuthorName: event.AuthorName,
			Message:    event.Message,
			CreatedAt:  event.CreatedAt,
		})
	}
	return events, nil
}

func toAuctionRoom(room queries.AuctionRoom) AuctionRoom {
	return AuctionRoom{
		Id:                  room.Id,
		Path:                url.URL{Path: "/rooms/" + strconv.Itoa(room.Id)},
		CreatorName:         room.CreatorName,
		Title:               room.Title,
		Subject:             room.Subject,
		SubjectDescription:  room.SubjectDescription,
		Status:              room.Status,
		StartingAmount:      room.StartingAmount,
		CurrentAmount:       room.CurrentAmount,
		HasWinningBid:       room.HasWinningBid,
		WinningBidderName:   room.WinningBidderName,
		CreatedAt:           room.CreatedAt,
		ParticipantCount:    room.ParticipantCount,
		ViewerIsCreator:     room.ViewerIsCreator,
		ViewerIsParticipant: room.ViewerIsParticipant,
	}
}

func FormatDollars(cents int) string {
	digits := strconv.Itoa(cents / 100)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "," + digits[index:]
	}
	return digits
}

func FormatCents(cents int) string {
	remainder := cents % 100
	if remainder < 10 {
		return "0" + strconv.Itoa(remainder)
	}
	return strconv.Itoa(remainder)
}

// AmountValue renders cents for a number input's value/min attribute: no
// grouping and no currency sign, so the browser reads it back as a number.
func AmountValue(cents int) string {
	return strconv.Itoa(cents/100) + "." + FormatCents(cents)
}

func FormatCreatedAt(createdAt time.Time) string {
	return createdAt.Local().Format("Jan 2, 15:04")
}

func BidPath(roomID int) url.URL {
	return url.URL{Path: "/rooms/" + strconv.Itoa(roomID) + "/bid"}
}

func MessagePath(roomID int) url.URL {
	return url.URL{Path: "/rooms/" + strconv.Itoa(roomID) + "/message"}
}

func ClosePath(roomID int) url.URL {
	return url.URL{Path: "/rooms/" + strconv.Itoa(roomID) + "/close"}
}
