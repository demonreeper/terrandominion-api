package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Faction bases, B4 (base-building-plan.md 5, 6, 7, 10): Collector Arrays tether free nodes inside the zone into the
// vault on the TN rules (the cooldown grows while no member comes by); members deposit into the vault on site; the
// Uplink takes members' uploads with no charge; Wardens and up read the VAULT LOG; a member on site keeps the base
// looked after and counts toward the "stand at your base" mission.

var lastEconomyTick time.Time

// economyTick runs once a minute from the base tick.
func (s *Service) economyTick(ctx context.Context) {
	now := s.now()
	if now.Sub(lastEconomyTick) < time.Minute {
		return
	}
	elapsed := now.Sub(lastEconomyTick)
	lastEconomyTick = now
	st := s.Settings(ctx)
	s.visitTick(ctx, st, now, elapsed)
	s.upkeepTick(ctx, st, now)
	s.collectorTick(ctx, st, now)
}

// visitTick: every base with a member of its faction on site is looked after now; each of those members gets the
// minute toward their stand-at-your-base mission.
func (s *Service) visitTick(ctx context.Context, st game.Settings, now time.Time, elapsed time.Duration) {
	on, err := store.MembersOnSiteAll(ctx, s.db, now.Add(-time.Duration(st.BasePresenceSec*float64(time.Second))))
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 visit tick failed", "err", err)
		}
		return
	}
	seen := map[int64]bool{}
	var bases []int64
	for _, o := range on {
		if !seen[o.BaseID] {
			seen[o.BaseID] = true
			bases = append(bases, o.BaseID)
		}
	}
	if err := store.MarkVisited(ctx, s.db, bases, now); err != nil {
		slog.Error("E1900 mark visited failed", "err", err)
	}
	minutes := max(1, int(elapsed.Minutes()+0.5))
	if minutes > 5 {
		minutes = 1 // the first tick after a restart counts one minute
	}
	for _, o := range on {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			return advanceMissionsAt(ctx, tx, o.PlayerID, o.BaseID, game.GoalBaseStand, "", minutes, st, now)
		})
		if err != nil {
			slog.Error("E1900 base stand mission failed", "player", o.PlayerID, "err", err)
		}
	}
}

// collectorTick tops up every base's collector links to what its arrays tether and harvests the due ones.
func (s *Service) collectorTick(ctx context.Context, st game.Settings, now time.Time) {
	bases, err := store.CollectorBases(ctx, s.db)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 collector tick failed", "err", err)
		}
		return
	}
	speed := map[int64]float64{}
	faction := map[int64]int{}
	for _, b := range bases {
		want := 0
		for _, mk := range b.Mks {
			want += game.CollectorLinks(mk, st)
		}
		speed[b.ID] = game.TetherSpeed(st, now.Sub(b.LastVisit))
		faction[b.ID] = b.FactionID
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			have, err := store.CollectorLinkCount(ctx, tx, b.ID)
			if err != nil {
				return err
			}
			if have > want {
				return store.TrimCollectorLinks(ctx, tx, b.ID, want)
			}
			if have == want {
				return nil
			}
			free, err := store.FreeNodesInZone(ctx, tx, b.Lat, b.Lon, b.ZoneM, st.NodeDepletedReserve, want-have)
			if err != nil {
				return err
			}
			for _, n := range free {
				if err := store.InsertCollectorLink(ctx, tx, b.ID, n.NodeID, now); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			slog.Error("E1900 collector links failed", "base", b.ID, "err", err)
		}
	}

	due, err := store.DueCollectorLinks(ctx, s.db, now, 500)
	if err != nil {
		slog.Error("E1900 collector due failed", "err", err)
		return
	}
	for _, link := range due {
		sp, ok := speed[link.BaseID]
		if !ok {
			// The base lost its arrays (offline, packed away): the trim above takes the link next time.
			continue
		}
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			node, err := store.NodeForUpdate(ctx, tx, link.NodeID)
			if err != nil {
				return err
			}
			harvest, ok := game.TetherHarvest{}, false
			if node.Status == "active" {
				harvest, ok = game.EvaluateTetherHarvest(st, node.Yield, node.Reserve)
			}
			if !ok {
				// Depleted or gone: let the node go; a free one is picked next minute.
				return store.DeleteCollectorLink(ctx, tx, link.NodeID)
			}
			if _, err := store.DrainNode(ctx, tx, node.ID, harvest.ReserveCost); err != nil {
				return err
			}
			amount := game.CollectorAmount(harvest.Amount, node.Resource, faction[link.BaseID], st)
			var delta [4]int64
			for i, r := range resourceOrder {
				if r == node.Resource {
					delta[i] = amount
				}
			}
			if err := store.AddVault(ctx, tx, link.BaseID, 0, 0, "collector", delta, now); err != nil {
				return err
			}
			next := now.Add(time.Duration(game.TetherCooldownSec(st, sp)) * time.Second)
			return store.SetCollectorLinkNext(ctx, tx, link.NodeID, next, amount)
		})
		if err != nil {
			slog.Error("E1900 collector harvest failed", "node", link.NodeID, "err", err)
		}
	}
}

// AtBaseRequest is a fix (and for a deposit, amounts) at a base.
type AtBaseRequest struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AccuracyM float64 `json:"accuracyM"`
	Alloy     int64   `json:"alloy"`
	Crystal   int64   `json:"crystal"`
	Data      int64   `json:"data"`
	Credit    int64   `json:"credit"`
}

// onSiteMember checks the caller stands inside the base's zone as a member of its faction and region.
func (s *Service) onSiteMember(ctx context.Context, tx *sql.Tx, playerID, baseID int64, req AtBaseRequest, st game.Settings) (*store.Membership, store.BaseRef, error) {
	m, b, err := s.baseMember(ctx, tx, playerID, baseID)
	if err != nil {
		return nil, b, err
	}
	if req.AccuracyM <= 0 || req.AccuracyM > st.BaseSpotAccuracyM || game.DistanceM(req.Lat, req.Lon, b.Lat, b.Lon) > b.ZoneM {
		return nil, b, apperr.NotOnSite
	}
	return m, b, nil
}

// DepositResult is what went into the vault and the points it earned.
type DepositResult struct {
	Given  store.Resources4 `json:"given"`
	Points int64            `json:"points"`
	Vault  store.Resources4 `json:"vault"`
}

// Deposit: a member gives from their storage straight into the base's vault, on site. It earns points like giving to a
// project (it can't be taken back).
func (s *Service) Deposit(ctx context.Context, playerID, baseID int64, req AtBaseRequest) (DepositResult, error) {
	var out DepositResult
	st := s.Settings(ctx)
	now := s.now()
	offer := [4]int64{req.Alloy, req.Crystal, req.Data, req.Credit}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		m, _, err := s.onSiteMember(ctx, tx, playerID, baseID, req, st)
		if err != nil {
			return err
		}
		if !game.Can(m.Rank, m.Nomad, game.PowerGive) {
			return apperr.ProjectRefused.Say("Your rank can't give.")
		}
		stored, err := store.Uploaded(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var given [4]int64
		for i, r := range resourceOrder {
			given[i] = max(0, min(offer[i], stored[r]))
		}
		if game.Total(given) == 0 {
			return apperr.NothingToGive.Say("You have none of that in storage.")
		}
		for i, r := range resourceOrder {
			if given[i] > 0 {
				if err := store.AddUploaded(ctx, tx, playerID, r, -given[i]); err != nil {
					return err
				}
				if err := advanceMissionsAt(ctx, tx, playerID, baseID, game.GoalBaseGive, r, int(given[i]), st, now); err != nil {
					return err
				}
			}
		}
		if err := store.AddVault(ctx, tx, baseID, playerID, 0, "deposit", given, now); err != nil {
			return err
		}
		out.Points = game.GivePoints(given, st)
		if err := store.InsertContribution(ctx, tx, baseID, 0, playerID, false, given, out.Points); err != nil {
			return err
		}
		if err := store.AddContributionPoints(ctx, tx, playerID, out.Points); err != nil {
			return err
		}
		v, err := store.VaultForUpdate(ctx, tx, baseID)
		if err != nil {
			return err
		}
		out.Given = store.Resources4{Alloy: given[0], Crystal: given[1], Data: given[2], Credit: given[3]}
		out.Vault = store.Resources4{Alloy: v[0], Crystal: v[1], Data: v[2], Credit: v[3]}
		return nil
	})
	return out, err
}

// UploadResult is what moved from carried to storage at the Uplink.
type UploadResult struct {
	Moved   map[string]int64 `json:"moved"`
	Carried map[string]int64 `json:"carried"`
	Stored  map[string]int64 `json:"stored"`
}

// UploadAtBase: a member on site uploads what they carry through the base's Uplink with no charge; it counts as a TN
// upload (missions).
func (s *Service) UploadAtBase(ctx context.Context, playerID, baseID int64, req AtBaseRequest) (UploadResult, error) {
	var out UploadResult
	st := s.Settings(ctx)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		if _, _, err := s.onSiteMember(ctx, tx, playerID, baseID, req, st); err != nil {
			return err
		}
		has, err := store.BaseHasModule(ctx, tx, baseID, "uplink")
		if err != nil {
			return err
		}
		if !has {
			return apperr.ProjectRefused.Say("This base has no Uplink online.")
		}
		carried, err := store.Carried(ctx, tx, playerID)
		if err != nil {
			return err
		}
		out.Moved = map[string]int64{}
		for r, amount := range carried {
			if amount <= 0 {
				continue
			}
			if err := store.AddCarried(ctx, tx, playerID, r, -amount); err != nil {
				return err
			}
			if err := store.AddUploaded(ctx, tx, playerID, r, amount); err != nil {
				return err
			}
			out.Moved[r] = amount
			carried[r] = 0
		}
		if len(out.Moved) == 0 {
			return apperr.TetherNothing
		}
		if err := advanceMissions(ctx, tx, playerID, game.GoalUpload, "", 1, st, s.now()); err != nil {
			return err
		}
		out.Carried = carried
		out.Stored, err = store.Uploaded(ctx, tx, playerID)
		return err
	})
	return out, err
}

// VaultView is GET /v1/bases/{id}/vault: the vault and its log, for Wardens and up of the base's faction and region.
type VaultView struct {
	Vault store.Resources4   `json:"vault"`
	Log   []store.VaultEntry `json:"log"`
}

func (s *Service) Vault(ctx context.Context, playerID, baseID int64) (VaultView, error) {
	var out VaultView
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		m, _, err := s.baseMember(ctx, tx, playerID, baseID)
		if err != nil {
			return err
		}
		if !game.Can(m.Rank, m.Nomad, game.PowerVaultAnywhere) {
			return apperr.ProjectRefused.Say("Only Wardens and above read the vault log.")
		}
		v, err := store.VaultForUpdate(ctx, tx, baseID)
		if err != nil {
			return err
		}
		out.Vault = store.Resources4{Alloy: v[0], Crystal: v[1], Data: v[2], Credit: v[3]}
		out.Log, err = store.VaultLog(ctx, tx, baseID, 60)
		return err
	})
	if errors.Is(err, store.ErrNotFound) {
		return out, apperr.BaseGone
	}
	return out, err
}
