package store

import (
	"context"

	"games.nvim/server/game"
)

type StatRecord struct {
	User                game.UserID
	Game                game.GameType
	Wins, Losses, Draws int
}

type HeadToHeadRecord struct {
	User, Opponent      game.UserID
	Game                game.GameType
	Wins, Losses, Draws int
}

const outcomeTallies = `
	COALESCE(SUM(a.outcome = 'win'), 0),
	COALESCE(SUM(a.outcome = 'loss'), 0),
	COALESCE(SUM(a.outcome = 'draw'), 0)`

func (s *Store) Stats(ctx context.Context) ([]StatRecord, []HeadToHeadRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.username, g.game_type,`+outcomeTallies+`
		FROM game_players a JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL
		GROUP BY a.username, g.game_type
		ORDER BY a.username, g.game_type`)
	if err != nil {
		return nil, nil, err
	}
	stats, err := collect(rows, func() (StatRecord, error) {
		var r StatRecord
		return r, rows.Scan(&r.User, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	})
	if err != nil {
		return nil, nil, err
	}

	rows, err = s.db.QueryContext(ctx, `
		SELECT a.username, b.username, g.game_type,`+outcomeTallies+`
		FROM game_players a
		JOIN game_players b ON b.game_id = a.game_id AND b.username <> a.username
		JOIN games g ON g.game_id = a.game_id
		WHERE a.outcome IS NOT NULL
		GROUP BY a.username, b.username, g.game_type
		ORDER BY a.username, b.username, g.game_type`)
	if err != nil {
		return nil, nil, err
	}
	headToHead, err := collect(rows, func() (HeadToHeadRecord, error) {
		var r HeadToHeadRecord
		return r, rows.Scan(&r.User, &r.Opponent, &r.Game, &r.Wins, &r.Losses, &r.Draws)
	})
	return stats, headToHead, err
}
