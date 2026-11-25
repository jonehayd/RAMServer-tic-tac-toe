// Package ultimatetictactoe is the RAMServer ruleset for Ultimate Tic-Tac-Toe.
//
// The encodings clients use are specified in README.md in this directory.
package ultimatetictactoe

import (
	"fmt"
	"strconv"

	"github.com/stevenlagoy/ramserver-core/server/game"
)

const Name = "ultimate_tictactoe"

const (
	ReasonThreeInARow      game.TerminationReason = "THREE_IN_A_ROW"
	ReasonDraw             game.TerminationReason = "DRAW"
	ReasonResignation      game.TerminationReason = "RESIGNATION"
	ReasonNotEnoughPlayers game.TerminationReason = "NOT_ENOUGH_PLAYERS"
)

const ResignPayload = "resign"

const (
	AnnotationXPlayer     = "x_player"
	AnnotationOPlayer     = "o_player"
	AnnotationMoves       = "moves"
	AnnotationLastMove    = "last_move"
	AnnotationWinningLine = "winning_line"
	AnnotationResigned    = "resigned"
	AnnotationLocalStatus = "local_status"
	AnnotationNextBoard   = "next_board"
)

const ViewLen = 93

const (
	cellEmpty = byte('.')
	markX     = byte('X')
	markO     = byte('O')

	localOpen = byte('.')
	localDraw = byte('D')

	statusXWon = byte('x')
	statusOWon = byte('o')
	statusDraw = byte('D')
	statusOver = byte('-')

	youSpectator = byte('S')
)

const (
	seatX      = 0
	seatO      = 1
	noSeat     = -1
	noBoard    = -1
	noCell     = -1
	numBoards  = 9
	numCells   = 9
	numPlayers = 2
)

var lines = [8][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
	{0, 4, 8}, {2, 4, 6},
}

var boardNames = [numBoards]string{
	"top-left", "top-center", "top-right",
	"middle-left", "center", "middle-right",
	"bottom-left", "bottom-center", "bottom-right",
}

var cellNames = [numCells]string{
	"top-left", "top-center", "top-right",
	"middle-left", "center", "middle-right",
	"bottom-left", "bottom-center", "bottom-right",
}

type Game struct{}

var (
	_ game.Game             = (*Game)(nil)
	_ game.OutOfTurnActions = (*Game)(nil)
	_ game.Annotator        = (*Game)(nil)
)

// State holds all data for a match. Its arrays and scalars make it safe to copy before applying actions.
type State struct {
	Players     [numPlayers]string
	Boards      [numBoards][numCells]byte
	LocalStatus [numBoards]byte
	Turn        int
	NextBoard   int // required board, or noBoard when any unfinished board is available
	Moves       int
	LastBoard   int
	LastCell    int
	Over        bool
	Winner      int
	Line        [3]int
	Resigned    int
	Reason      game.TerminationReason
}

func (*Game) Roles() []game.RoleSpec {
	return []game.RoleSpec{
		{Name: game.RolePlayer, Min: 2, Max: 2},
		{Name: game.RoleSpectator, Min: 0, Max: 0},
	}
}

func (*Game) ReadyToStart(roster map[string]game.Role, ready map[string]bool) bool {
	players := 0
	for id, role := range roster {
		if role != game.RolePlayer {
			continue
		}
		if !ready[id] {
			return false
		}
		players++
	}
	return players == numPlayers
}

func (*Game) NewState(rosterOrder []string, roles map[string]game.Role) game.GameState {
	s := State{Turn: seatX, NextBoard: noBoard, LastBoard: noBoard, LastCell: noCell, Winner: noSeat, Resigned: noSeat}
	for board := range s.Boards {
		for cell := range s.Boards[board] {
			s.Boards[board][cell] = cellEmpty
		}
		s.LocalStatus[board] = localOpen
	}
	seated := 0
	for _, id := range rosterOrder {
		if roles[id] == game.RolePlayer && seated < numPlayers {
			s.Players[seated] = id
			seated++
		}
	}
	if seated < numPlayers {
		s.Over = true
		s.Reason = ReasonNotEnoughPlayers
	}
	return s
}

func stateOf(st game.GameState) (State, bool) {
	switch s := st.(type) {
	case State:
		return s, true
	case *State:
		if s != nil {
			return *s, true
		}
	}
	return State{}, false
}

func (s State) seatOf(memberID string) int {
	switch {
	case memberID == "":
		return noSeat
	case memberID == s.Players[seatX]:
		return seatX
	case memberID == s.Players[seatO]:
		return seatO
	default:
		return noSeat
	}
}

func markOf(seat int) byte {
	if seat == seatO {
		return markO
	}
	return markX
}

func (*Game) ActiveTurn(st game.GameState) []string {
	s, ok := stateOf(st)
	if !ok || s.Over {
		return []string{}
	}
	return []string{s.Players[s.Turn]}
}

func (*Game) LegalActions(st game.GameState, memberID string) []game.ActionSpec {
	s, ok := stateOf(st)
	if !ok || s.Over {
		return []game.ActionSpec{}
	}
	seat := s.seatOf(memberID)
	if seat == noSeat {
		return []game.ActionSpec{}
	}
	var specs []game.ActionSpec
	if seat == s.Turn {
		for board := 0; board < numBoards; board++ {
			if s.NextBoard != noBoard && board != s.NextBoard {
				continue
			}
			if s.LocalStatus[board] != localOpen {
				continue
			}
			for cell, mark := range s.Boards[board] {
				if mark != cellEmpty {
					continue
				}
				payload := []byte{byte('0' + board), byte('0' + cell)}
				specs = append(specs, game.ActionSpec{
					Payload:     payload,
					Description: fmt.Sprintf("%s board, %s", boardNames[board], cellNames[cell]),
				})
			}
		}
	}
	specs = append(specs, game.ActionSpec{Payload: []byte(ResignPayload), Description: "resign"})
	return specs
}

type parsedAction struct {
	board  int
	cell   int
	resign bool
}

func parse(payload []byte) (parsedAction, error) {
	if len(payload) == 2 && payload[0] >= '0' && payload[0] <= '8' && payload[1] >= '0' && payload[1] <= '8' {
		return parsedAction{board: int(payload[0] - '0'), cell: int(payload[1] - '0')}, nil
	}
	if string(payload) == ResignPayload {
		return parsedAction{board: noBoard, cell: noCell, resign: true}, nil
	}
	return parsedAction{}, fmt.Errorf("%w: send two digits from '0' to '8' for board and cell, or %q", game.ErrMalformedAction, ResignPayload)
}

func (*Game) Validate(st game.GameState, action game.Action) error {
	a, err := parse(action.Payload)
	if err != nil {
		return err
	}
	s, ok := stateOf(st)
	if !ok {
		return fmt.Errorf("%w: no game in progress", game.ErrIllegalAction)
	}
	if s.Over {
		return fmt.Errorf("%w: the game is over", game.ErrIllegalAction)
	}
	seat := s.seatOf(action.ActorID)
	if seat == noSeat {
		return fmt.Errorf("%w: spectators cannot play", game.ErrIllegalAction)
	}
	if a.resign {
		return nil
	}
	if seat != s.Turn {
		return fmt.Errorf("%w: it is not your turn", game.ErrIllegalAction)
	}
	if s.NextBoard != noBoard && a.board != s.NextBoard {
		return fmt.Errorf("%w: you must play in %s board", game.ErrIllegalAction, boardNames[s.NextBoard])
	}
	if s.LocalStatus[a.board] != localOpen {
		return fmt.Errorf("%w: %s board is already finished", game.ErrIllegalAction, boardNames[a.board])
	}
	if s.Boards[a.board][a.cell] != cellEmpty {
		return fmt.Errorf("%w: cell %d in %s board is already taken", game.ErrIllegalAction, a.cell, boardNames[a.board])
	}
	return nil
}

func (*Game) AllowsOutOfTurn(_ game.GameState, action game.Action) bool {
	return string(action.Payload) == ResignPayload
}

func (*Game) Apply(st game.GameState, action game.Action) game.GameState {
	s, ok := stateOf(st)
	if !ok {
		return st
	}
	a, err := parse(action.Payload)
	if err != nil || s.Over {
		return s
	}
	seat := s.seatOf(action.ActorID)
	if seat == noSeat {
		return s
	}
	if a.resign {
		s.Over = true
		s.NextBoard = noBoard
		s.Resigned = seat
		s.Winner = 1 - seat
		s.Reason = ReasonResignation
		return s
	}

	s.Boards[a.board][a.cell] = markOf(seat)
	s.Moves++
	s.LastBoard = a.board
	s.LastCell = a.cell
	if _, won := winningLine(s.Boards[a.board]); won {
		s.LocalStatus[a.board] = markOf(seat)
	} else if boardFull(s.Boards[a.board]) {
		s.LocalStatus[a.board] = localDraw
	}

	if line, won := winningLine(s.globalBoard()); won {
		s.Over = true
		s.NextBoard = noBoard
		s.Winner = seat
		s.Line = line
		s.Reason = ReasonThreeInARow
	} else if allBoardsFinished(s.LocalStatus) {
		s.Over = true
		s.NextBoard = noBoard
		s.Reason = ReasonDraw
	} else {
		s.Turn = 1 - seat
		s.NextBoard = a.cell
		if s.LocalStatus[s.NextBoard] != localOpen {
			s.NextBoard = noBoard
		}
	}
	return s
}

func winningLine(board [numCells]byte) ([3]int, bool) {
	for _, line := range lines {
		if mark := board[line[0]]; mark != cellEmpty && board[line[1]] == mark && board[line[2]] == mark {
			return line, true
		}
	}
	return [3]int{}, false
}

func boardFull(board [numCells]byte) bool {
	for _, mark := range board {
		if mark == cellEmpty {
			return false
		}
	}
	return true
}

func allBoardsFinished(status [numBoards]byte) bool {
	for _, result := range status {
		if result == localOpen {
			return false
		}
	}
	return true
}

func (s State) globalBoard() [numCells]byte {
	var board [numCells]byte
	for i, result := range s.LocalStatus {
		if result == markX || result == markO {
			board[i] = result
		} else {
			board[i] = cellEmpty
		}
	}
	return board
}

func (*Game) Outcome(st game.GameState) (game.MatchResult, bool) {
	s, ok := stateOf(st)
	if !ok || !s.Over {
		return nil, false
	}
	var winners []string
	if s.Winner != noSeat {
		winners = []string{s.Players[s.Winner]}
	}
	return game.Result{WinnerIDs: winners, Reason: s.Reason}, true
}

func (*Game) ViewFor(st game.GameState, memberID string) []byte {
	s, ok := stateOf(st)
	if !ok {
		return []byte{}
	}
	view := make([]byte, 0, ViewLen)
	for _, board := range s.Boards {
		view = append(view, board[:]...)
	}
	view = append(view, s.LocalStatus[:]...)
	if s.NextBoard == noBoard {
		view = append(view, '.')
	} else {
		view = append(view, byte('0'+s.NextBoard))
	}
	view = append(view, s.statusByte())
	if seat := s.seatOf(memberID); seat == noSeat {
		view = append(view, youSpectator)
	} else {
		view = append(view, markOf(seat))
	}
	return view
}

func (s State) statusByte() byte {
	switch {
	case !s.Over:
		return markOf(s.Turn)
	case s.Winner == seatX:
		return statusXWon
	case s.Winner == seatO:
		return statusOWon
	case s.Reason == ReasonDraw:
		return statusDraw
	default:
		return statusOver
	}
}

func (*Game) Annotations(st game.GameState, _ string) map[string]string {
	s, ok := stateOf(st)
	if !ok {
		return map[string]string{}
	}
	localStatus := string(s.LocalStatus[:])
	nextBoard := "."
	if s.NextBoard != noBoard {
		nextBoard = strconv.Itoa(s.NextBoard)
	}
	annotations := map[string]string{
		AnnotationXPlayer:     s.Players[seatX],
		AnnotationOPlayer:     s.Players[seatO],
		AnnotationMoves:       strconv.Itoa(s.Moves),
		AnnotationLocalStatus: localStatus,
		AnnotationNextBoard:   nextBoard,
	}
	if s.LastBoard != noBoard {
		annotations[AnnotationLastMove] = fmt.Sprintf("%d,%d", s.LastBoard, s.LastCell)
	}
	if s.Reason == ReasonThreeInARow {
		annotations[AnnotationWinningLine] = fmt.Sprintf("%d,%d,%d", s.Line[0], s.Line[1], s.Line[2])
	}
	if s.Resigned != noSeat {
		annotations[AnnotationResigned] = string(markOf(s.Resigned))
	}
	return annotations
}
