package service

import (
	"context"

	"github.com/demonreeper/terrandominion/api/internal/store"
)

const noticeListLimit = 50

// Notices lists what happened to the player after a notice id, newest first.
func (s *Service) Notices(ctx context.Context, playerID, after int64) ([]store.Notice, error) {
	return store.Notices(ctx, s.db, playerID, after, noticeListLimit)
}
