package store

import (
	"context"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

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
// user is "", and the whole leaderboard either way, which leaves out the games
// played alone, ranked as protocol.StatsResult says; HeadToHead is by user,
// opponent and game.
func (s *Store) Stats(ctx context.Context, user game.UserID) (protocol.StatsResult, error) {
	records, err := queryAll(ctx, s.reader, func(row scanner) (protocol.StatsRecord, error) {
		var r protocol.StatsRecord
		return r, row.Scan(&r.User, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username, g.game_type,`+outcomeTallies+`
		FROM game_players a JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL AND `+ofUser+`
		GROUP BY a.username, g.game_type
		ORDER BY g.game_type, `+byRank, user, user)
	if err != nil {
		return protocol.StatsResult{}, err
	}
	headToHead, err := queryAll(ctx, s.reader, func(row scanner) (protocol.HeadToHeadRecord, error) {
		var r protocol.HeadToHeadRecord
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
		return protocol.StatsResult{}, err
	}
	leaderboard, err := queryAll(ctx, s.reader, func(row scanner) (protocol.LeaderboardEntry, error) {
		var r protocol.LeaderboardEntry
		return r, row.Scan(&r.User, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username,`+outcomeTallies+`
		FROM game_players a
		WHERE a.outcome IS NOT NULL
		AND (SELECT COUNT(*) FROM game_players p WHERE p.game_id = a.game_id) > 1
		GROUP BY a.username
		ORDER BY `+byRank)
	if err != nil {
		return protocol.StatsResult{}, err
	}
	return protocol.StatsResult{
		Records:     xslices.Clone(records),
		HeadToHead:  xslices.Clone(headToHead),
		Leaderboard: xslices.Clone(leaderboard),
	}, nil
}
