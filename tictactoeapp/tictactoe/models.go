package tictactoe

import (
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/user"
)

const (
	TIME_LAYOUT = "Jan 2, 2006 15:04:05 AM"
	SCORE_KIND  = "Score"
)

type Score struct {
	key     *datastore.Key
	Outcome string    `datastore:"outcome"`
	Played  time.Time `datastore:"played"`
	Player  string    `datastore:"player"`
}

func (s *Score) toMessage(msg *ScoreRespMsg) *ScoreRespMsg {
	if msg == nil {
		msg = &ScoreRespMsg{}
	}
	msg.Id = s.key.IntID()
	msg.Outcome = s.Outcome
	msg.Played = s.timestamp()
	return msg
}

func (s *Score) timestamp() string {
	return s.Played.Format(TIME_LAYOUT)
}

func (s *Score) put(c appengine.Context) (err error) {
	key := s.key
	if key == nil {
		key = datastore.NewIncompleteKey(c, SCORE_KIND, nil)
	}
	key, err = datastore.Put(c, key, s)
	if err == nil {
		s.key = key
	}
	return
}

func newScore(outcome string, u *user.User) *Score {
	return &Score{Outcome: outcome, Played: time.Now(), Player: userId(u)}
}

func newUserScoreQuery(u *user.User) *datastore.Query {
	return datastore.NewQuery(SCORE_KIND).Filter("player =", userId(u))
}

func fetchScores(c appengine.Context, q *datastore.Query, limit int) (
	[]*Score, error) {

	scores := make([]*Score, 0, limit)
	keys, err := q.Limit(limit).GetAll(c, &scores)
	if err != nil {
		return nil, err
	}
	for i, score := range scores {
		score.key = keys[i]
	}
	return scores, nil
}

func userId(u *user.User) string {
	return u.ID
}
