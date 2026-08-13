-- +goose Up
ALTER TABLE rooms
	ADD COLUMN starting_amount INTEGER NOT NULL DEFAULT 0
		CHECK (starting_amount >= 0);

CREATE TABLE auction_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	room_id INTEGER NOT NULL,
	event_type TEXT NOT NULL
		CHECK (event_type IN ('bid', 'host_message')),
	current_amount INTEGER,
	bidder_account_id TEXT,
	host_account_id TEXT,
	message TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE,
	FOREIGN KEY (bidder_account_id) REFERENCES accounts (id),
	FOREIGN KEY (host_account_id) REFERENCES accounts (id),
	CHECK (
		(
			event_type = 'bid'
			AND current_amount IS NOT NULL
			AND current_amount > 0
			AND bidder_account_id IS NOT NULL
			AND host_account_id IS NULL
			AND message IS NULL
		)
		OR
		(
			event_type = 'host_message'
			AND current_amount IS NULL
			AND bidder_account_id IS NULL
			AND host_account_id IS NOT NULL
			AND message IS NOT NULL
			AND length(trim(message)) > 0
		)
	)
);

CREATE INDEX auction_history_room_timeline
	ON auction_history (room_id, id);

CREATE INDEX auction_history_room_bid_amount_desc
	ON auction_history (room_id, current_amount DESC, id DESC)
	WHERE event_type = 'bid';

-- +goose StatementBegin
CREATE TRIGGER auction_history_room_must_be_open
BEFORE INSERT ON auction_history
WHEN NOT EXISTS (
	SELECT 1
	FROM rooms
	WHERE id = NEW.room_id
		AND status = 'open'
)
BEGIN
	SELECT RAISE(ABORT, 'auction room is not open');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER auction_history_host_must_own_room
BEFORE INSERT ON auction_history
WHEN NEW.event_type = 'host_message'
	AND NOT EXISTS (
		SELECT 1
		FROM rooms
		WHERE id = NEW.room_id
			AND creator_account_id = NEW.host_account_id
	)
BEGIN
	SELECT RAISE(ABORT, 'only the room host can post a message');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER auction_history_bidder_must_be_participant
BEFORE INSERT ON auction_history
WHEN NEW.event_type = 'bid'
	AND NOT EXISTS (
		SELECT 1
		FROM room_participants AS participant
		JOIN rooms AS room ON room.id = participant.room_id
		WHERE participant.room_id = NEW.room_id
			AND participant.account_id = NEW.bidder_account_id
			AND room.creator_account_id <> NEW.bidder_account_id
	)
BEGIN
	SELECT RAISE(ABORT, 'only a room participant can place a bid');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER auction_history_bid_must_increase
BEFORE INSERT ON auction_history
WHEN NEW.event_type = 'bid'
	AND NEW.current_amount <= COALESCE(
		(
			SELECT MAX(history.current_amount)
			FROM auction_history AS history
			WHERE history.room_id = NEW.room_id
				AND history.event_type = 'bid'
		),
		(
			SELECT room.starting_amount
			FROM rooms AS room
			WHERE room.id = NEW.room_id
		)
	)
BEGIN
	SELECT RAISE(ABORT, 'bid must be greater than the current amount');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER auction_history_bid_must_increase;
DROP TRIGGER auction_history_bidder_must_be_participant;
DROP TRIGGER auction_history_host_must_own_room;
DROP TRIGGER auction_history_room_must_be_open;
DROP TABLE auction_history;
ALTER TABLE rooms DROP COLUMN starting_amount;
