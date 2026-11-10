package service

import (
	"context"

	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

const noticeListLimit = 50

// Notices lists what happened to the player after a notice id, newest first.
func (s *Service) Notices(ctx context.Context, playerID, after int64) ([]store.Notice, error) {
	list, err := store.Notices(ctx, s.db, playerID, after, noticeListLimit)
	for i := range list {
		list[i].Severity = game.NoticeSeverity(list[i].Kind)
	}
	return list, err
}
