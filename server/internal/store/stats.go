package store

import (
	"context"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type Tally struct {
	Wins, Losses, Draws int
}

type StatRecord struct {
	User game.UserID
	Game game.GameType
	Tally
}

type HeadToHeadRecord struct {
	User, Opponent game.UserID
	Game           game.GameType
	Tally
}

type UserTally struct {
	User game.UserID
	Tally
}

// Stats holds Records grouped by game and ranked within each, HeadToHead by
// user, opponent and game, and Leaderboard every user's totals over all games,
// ranked. A rank orders most wins, then fewest losses, then most draws, then user.
type Stats struct {
	Records     []StatRecord
	HeadToHead  []HeadToHeadRecord
	Leaderboard []UserTally
}

const (
	outcomeTallies = `
	COALESCE(SUM(a.outcome = 'win'), 0) AS wins,
	COALESCE(SUM(a.outcome = 'loss'), 0) AS losses,
	COALESCE(SUM(a.outcome = 'draw'), 0) AS draws`
	byRank = `wins DESC, losses, draws DESC, a.username`
	// ofUser keeps every row when its two parameters are "".
	ofUser = `(? = '' OR a.username = ?)`
)

// Stats reads the records and head-to-head of user alone, or of everyone when
// user is "", and the whole leaderboard either way.
func (s *Store) Stats(ctx context.Context, user game.UserID) (Stats, error) {
	records, err := queryAll(ctx, s.reader, func(row scanner) (StatRecord, error) {
		var r StatRecord
		return r, row.Scan(&r.User, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username, g.game_type,`+outcomeTallies+`
		FROM game_players a JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL AND `+ofUser+`
		GROUP BY a.username, g.game_type
		ORDER BY g.game_type, `+byRank, user, user)
	if err != nil {
		return Stats{}, err
	}
	headToHead, err := queryAll(ctx, s.reader, func(row scanner) (HeadToHeadRecord, error) {
		var r HeadToHeadRecord
		return r, row.Scan(&r.User, &r.Opponent, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username, b.username, g.game_type,`+outcomeTallies+`
		FROM game_players a
		JOIN game_players b ON b.game_id = a.game_id AND b.username <> a.username
		JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL AND `+ofUser+`
		GROUP BY a.username, b.username, g.game_type
		ORDER BY a.username, b.username, g.game_type`, user, user)
	if err != nil {
		return Stats{}, err
	}
	leaderboard, err := queryAll(ctx, s.reader, func(row scanner) (UserTally, error) {
		var r UserTally
		return r, row.Scan(&r.User, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username,`+outcomeTallies+`
		FROM game_players a
		WHERE a.outcome IS NOT NULL
		GROUP BY a.username
		ORDER BY `+byRank)
	return Stats{Records: records, HeadToHead: headToHead, Leaderboard: leaderboard}, err
}
