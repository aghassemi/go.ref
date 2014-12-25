package main

import (
	"errors"

	"v.io/apps/rps"
	"v.io/veyron/veyron2"
	"v.io/veyron/veyron2/ipc"
	"v.io/veyron/veyron2/vlog"
)

// RPS implements rps.RockPaperScissorsServerMethods
type RPS struct {
	player      *Player
	judge       *Judge
	scoreKeeper *ScoreKeeper
}

func NewRPS() *RPS {
	return &RPS{player: NewPlayer(), judge: NewJudge(), scoreKeeper: NewScoreKeeper()}
}

func (r *RPS) Judge() *Judge {
	return r.judge
}

func (r *RPS) Player() *Player {
	return r.player
}

func (r *RPS) ScoreKeeper() *ScoreKeeper {
	return r.scoreKeeper
}

func (r *RPS) CreateGame(ctx ipc.ServerContext, opts rps.GameOptions) (rps.GameID, error) {
	vlog.VI(1).Infof("CreateGame %+v from %v", opts, ctx.RemoteBlessings().ForContext(ctx))
	names := ctx.LocalBlessings().ForContext(ctx)
	if len(names) == 0 {
		return rps.GameID{}, errors.New("no names provided for context")
	}
	return r.judge.createGame(names[0], opts)
}

func (r *RPS) Play(ctx rps.JudgePlayContext, id rps.GameID) (rps.PlayResult, error) {
	vlog.VI(1).Infof("Play %+v from %v", id, ctx.RemoteBlessings().ForContext(ctx))
	names := ctx.RemoteBlessings().ForContext(ctx)
	if len(names) == 0 {
		return rps.PlayResult{}, errors.New("no names provided for context")
	}
	return r.judge.play(ctx, names[0], id)
}

func (r *RPS) Challenge(ctx ipc.ServerContext, address string, id rps.GameID, opts rps.GameOptions) error {
	vlog.VI(1).Infof("Challenge (%q, %+v, %+v) from %v", address, id, opts, ctx.RemoteBlessings().ForContext(ctx))
	return r.player.challenge(veyron2.RuntimeFromContext(ctx.Context()), address, id, opts)
}

func (r *RPS) Record(ctx ipc.ServerContext, score rps.ScoreCard) error {
	vlog.VI(1).Infof("Record (%+v) from %v", score, ctx.RemoteBlessings().ForContext(ctx))
	return r.scoreKeeper.Record(ctx, score)
}
