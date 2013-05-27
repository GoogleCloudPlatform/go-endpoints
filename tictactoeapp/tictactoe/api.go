package tictactoe

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"appengine"
	"appengine/user"

	"github.com/crhym3/go-endpoints/endpoints"
)

const (
	clientId  = "YOUR-CLIENT-ID" // xxx.apps.googleusercontent.com
	authScope = "https://www.googleapis.com/auth/userinfo.email"
)

var defaultScopes = []string{authScope}
var clientIds = []string{clientId, endpoints.ApiExplorerClientId}

type BoardMsg struct {
	State string `json:"state" endpoints:"required"`
}

type ScoreReqMsg struct {
	Outcome string `json:"outcome" endpoints:"required"`
}

type ScoreRespMsg struct {
	Id      int64  `json:"id"`
	Outcome string `json:"outcome"`
	Played  string `json:"played"`
}

type ScoresListReq struct {
	Limit int `json:"limit"`
}

type ScoresListResp struct {
	Items []*ScoreRespMsg `json:"items"`
}

type TicTacToeApi struct {
}

func (ttt *TicTacToeApi) BoardGetMove(r *http.Request,
	req *BoardMsg, resp *BoardMsg) error {

	const boardLen = 9
	if len(req.State) != boardLen {
		return fmt.Errorf("Bad Request: Invalid board: %q", req.State)
	}
	runes := []rune(req.State)
	freeIndices := make([]int, 0)
	for pos, r := range runes {
		if r != 'O' && r != 'X' && r != '-' {
			return fmt.Errorf("Bad Request: Invalid rune: %q", r)
		}
		if r == '-' {
			freeIndices = append(freeIndices, pos)
		}
	}
	freeIdxLen := len(freeIndices)
	if freeIdxLen > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		randomIdx := r.Intn(freeIdxLen)
		runes[randomIdx] = 'O'
		resp.State = string(runes)
	} else {
		return fmt.Errorf("Bad Request: This board is full: %q", req.State)
	}
	return nil
}

func (ttt *TicTacToeApi) ScoresList(r *http.Request,
	req *ScoresListReq, resp *ScoresListResp) error {

	c := appengine.NewContext(r)
	u, err := getCurrentUser(c)
	if err != nil {
		return err
	}
	q := newUserScoreQuery(u)
	if req.Limit <= 0 {
		req.Limit = 10
	}
	scores, err := fetchScores(c, q, req.Limit)
	if err != nil {
		return err
	}
	resp.Items = make([]*ScoreRespMsg, len(scores))
	for i, score := range scores {
		resp.Items[i] = score.toMessage(nil)
	}
	return nil
}

func (ttt *TicTacToeApi) ScoresInsert(r *http.Request,
	req *ScoreReqMsg, resp *ScoreRespMsg) error {

	c := appengine.NewContext(r)
	u, err := getCurrentUser(c)
	if err != nil {
		return err
	}
	score := newScore(req.Outcome, u)
	if err := score.put(c); err != nil {
		return err
	}
	score.toMessage(resp)
	return nil
}

func getCurrentUser(c appengine.Context) (*user.User, error) {
	u, err := endpoints.CurrentUserWithScope(c, authScope)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("Unauthorized: Please, sign in.")
	}
	return u, nil
}

func init() {
	api := &TicTacToeApi{}
	rpcService, err := endpoints.RegisterService(api,
		"tictactoe", "v1", "Tic Tac Toe API", true)
	if err != nil {
		panic(err.Error())
	}

	info := rpcService.MethodByName("BoardGetMove").Info()
	info.Path, info.HttpMethod, info.Name, info.Scopes, info.ClientIds =
		"board", "POST", "board.getmove", defaultScopes, clientIds

	info = rpcService.MethodByName("ScoresList").Info()
	info.Path, info.HttpMethod, info.Name, info.Scopes, info.ClientIds =
		"scores", "GET", "scores.list", defaultScopes, clientIds

	info = rpcService.MethodByName("ScoresInsert").Info()
	info.Path, info.HttpMethod, info.Name, info.Scopes, info.ClientIds =
		"scores", "POST", "scores.insert", defaultScopes, clientIds

	endpoints.HandleHttp()
}
