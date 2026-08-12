package pages

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
	"github.com/shibukawa/popcornwave/pwpage"
)

func Load(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	if _, err := queries.UpsertAccount(r.Context(), user.AccountID, user.DisplayName, user.Email); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	route, err := DecodeRoute(r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	_ = route
	params := PageParams{
		ViewerAccountID: user.AccountID,
		DisplayName:     user.DisplayName,
		Email:           user.Email,
		LogoutPath:      url.URL{Path: "/auth/logout"},
		Live:            pw.WantsLive(r),
	}
	wrappers := []pwpage.Wrapper{BindLayout(LayoutParams{})}
	if err := pwpage.Render(w, r, wrappers, Page(params)); err != nil {
		pw.WriteProblem(w, r, err)
	}
}

func WatchRooms(ctx context.Context, viewerAccountID string) iter.Seq2[RoomList, error] {
	return func(yield func(RoomList, error) bool) {
		changed, unsubscribe := lobby.Subscribe()
		defer unsubscribe()

		for {
			rooms, err := loadRooms(ctx, viewerAccountID)
			if !yield(RoomList{Rooms: rooms, HasRooms: len(rooms) > 0}, err) {
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

func loadRooms(ctx context.Context, viewerAccountID string) ([]Room, error) {
	rooms := make([]Room, 0)
	for room, err := range queries.ListOpenRooms(ctx, viewerAccountID) {
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, Room{
			Id:                 room.Id,
			Path:               RoomPath(room.Id),
			CreatorName:        room.CreatorDisplayName,
			Title:              room.Title,
			Subject:            room.Subject,
			SubjectDescription: room.SubjectDescription,
			StartingAmount:     room.StartingAmount,
			CreatedAt:          room.CreatedAt,
			ParticipantCount:   room.ParticipantCount,
			IsCreator:          room.IsCreator,
			IsParticipant:      room.IsParticipant,
		})
	}
	return rooms, nil
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

func FormatCreatedAt(createdAt time.Time) string {
	return createdAt.Local().Format("Jan 2, 15:04")
}

func RoomPath(roomID int) url.URL {
	return url.URL{Path: "/rooms/" + strconv.Itoa(roomID)}
}
