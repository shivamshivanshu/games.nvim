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

const outcomeTallies = `
	COALESCE(SUM(a.outcome = 'win'), 0),
	COALESCE(SUM(a.outcome = 'loss'), 0),
	COALESCE(SUM(a.outcome = 'draw'), 0)`

func (s *Store) Stats(ctx context.Context) ([]StatRecord, []HeadToHeadRecord, error) {
	stats, err := queryAll(ctx, s.db, func(row scanner) (StatRecord, error) {
		var r StatRecord
		return r, row.Scan(&r.User, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username, g.game_type,`+outcomeTallies+`
		FROM game_players a JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL
		GROUP BY a.username, g.game_type
		ORDER BY a.username, g.game_type`)
	if err != nil {
		return nil, nil, err
	}
	headToHead, err := queryAll(ctx, s.db, func(row scanner) (HeadToHeadRecord, error) {
		var r HeadToHeadRecord
		return r, row.Scan(&r.User, &r.Opponent, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	}, `
		SELECT a.username, b.username, g.game_type,`+outcomeTallies+`
		FROM game_players a
		JOIN game_players b ON b.game_id = a.game_id AND b.username <> a.username
		JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL
		GROUP BY a.username, b.username, g.game_type
		ORDER BY a.username, b.username, g.game_type`)
	return stats, headToHead, err
}
