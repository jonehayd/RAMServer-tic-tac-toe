// Command fakeserver is a stand-in RAMServer for the client's integration tests, written from
// docs/protocol.md, docs/framing.md and docs/writing-a-ruleset.md alone. It is not the real server and
// makes no claim to match it beyond what those documents say: it exists so two clients can play complete
// matches in `make test` without a server build.
//
// It speaks the protocol over plain TCP and runs the real tic-tac-toe rulesets through the game.Game
// interface, the way the documents describe the server doing it. It sends heartbeats every -heartbeat
// and counts the clients' heartbeat replies, printing totals when it gets SIGTERM or SIGINT.
//
// Protobuf is encoded by hand (protowire.go) so the module needs no dependencies.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/stevenlagoy/ramserver-core/server/game"

	"ramserver-tictactoe/rulesets/tictactoe"
	"ramserver-tictactoe/rulesets/ultimatetictactoe"
)

const maxPayload = 65536

// Error codes, session statuses and end reasons from proto/ramserver/v1/ramserver.proto
const (
	errInvalidProtocolVersion = 1
	errAuthFailed             = 2
	errUnknownGame            = 3
	errSessionNotFound        = 4
	errSessionNotJoinable     = 5
	errOutOfTurn              = 6
	errIllegalAction          = 7
	errMalformed              = 8
	errInvalidState           = 9
	errNotInSession           = 10
	errAlreadyInSession       = 11
	errInvalidArgument        = 12
	errStaleState             = 13

	statusWaiting    = 1
	statusInProgress = 4

	endCompleted = 1
)

var (
	heartbeatsSent     atomic.Int64
	heartbeatsReceived atomic.Int64
	matchesFinished    atomic.Int64
	actionsRejected    atomic.Int64
)

type conn struct {
	net.Conn
	out    chan []byte // complete frames
	done   chan struct{}
	once   sync.Once
	phase  int // 0: new, 1: connected, 2: authenticated
	player *player
}

func (c *conn) close() {
	c.once.Do(func() {
		close(c.done)
		c.Conn.Close()
	})
}

// send queues one ServerMessage (or a heartbeat, for nil)
func (c *conn) send(payload []byte) {
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)
	select {
	case c.out <- frame:
	case <-c.done:
	default: // a client that stops reading is dropped, as the real server does after 32 messages
		c.close()
	}
}

type player struct {
	id, name string
	conn     *conn
	session  *session
}

type session struct {
	id                 string
	gameID             string
	rules              game.Game
	private            bool
	members            []*player
	roles              map[string]game.Role
	ready              map[string]bool
	state              game.GameState
	playing            bool
	version            uint64
	turns              uint32
	history            [][]byte // encoded MatchResult messages
	accepted, rejected map[string]int
}

type server struct {
	mu       sync.Mutex
	games    map[string]game.Game
	sessions []*session
	nextID   int
}

func newID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "address to listen on")
	hb := flag.Duration("heartbeat", time.Second, "heartbeat interval")
	flag.Parse()
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("listening %s\n", ln.Addr())
	os.Stdout.Sync()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		fmt.Printf("stats heartbeats_sent=%d heartbeats_received=%d matches=%d rejected=%d\n",
			heartbeatsSent.Load(), heartbeatsReceived.Load(), matchesFinished.Load(), actionsRejected.Load())
		os.Exit(0)
	}()

	s := &server{games: map[string]game.Game{
		tictactoe.Name:         &tictactoe.Game{},
		ultimatetictactoe.Name: &ultimatetictactoe.Game{},
	}}
	for {
		nc, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		c := &conn{Conn: nc, out: make(chan []byte, 32), done: make(chan struct{})}
		go c.writer(*hb)
		go s.reader(c)
	}
}

func (c *conn) writer(hb time.Duration) {
	t := time.NewTicker(hb)
	defer t.Stop()
	for {
		select {
		case f := <-c.out:
			if _, err := c.Write(f); err != nil {
				c.close()
				return
			}
		case <-t.C:
			heartbeatsSent.Add(1)
			if _, err := c.Write([]byte{0, 0, 0, 0}); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (s *server) reader(c *conn) {
	defer func() {
		c.close()
		s.mu.Lock()
		s.disconnect(c)
		s.mu.Unlock()
	}()
	r := bufio.NewReader(c)
	c.SetReadDeadline(time.Now().Add(5 * time.Second)) // first frame within 5 s
	for {
		var prefix [4]byte
		if _, err := io.ReadFull(r, prefix[:]); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(prefix[:])
		if n > maxPayload {
			log.Printf("oversized frame (%d bytes): closing", n)
			return
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			return
		}
		c.SetReadDeadline(time.Now().Add(30 * time.Second)) // some frame at least every 30 s
		if n == 0 {
			heartbeatsReceived.Add(1)
			continue
		}
		s.mu.Lock()
		s.handle(c, payload)
		s.mu.Unlock()
	}
}

func (s *server) disconnect(c *conn) {
	// Resuming isn't supported (as docs/writing-a-client.md says of the real server); a dropped player
	// leaves their session.
	if p := c.player; p != nil && p.session != nil {
		s.leave(p)
	}
}

func reply(c *conn, seq uint64, field int, body []byte) {
	var m enc
	m.uint(1, seq)
	m.msg(field, body)
	c.send(m)
}

func replyError(c *conn, seq uint64, code int, msg string, sessionID string) {
	var e enc
	e.uint(1, uint64(code))
	e.str(2, msg)
	e.str(3, sessionID)
	reply(c, seq, 5, e)
}

func (s *server) handle(c *conn, payload []byte) {
	msg, err := parse(payload)
	if err != nil {
		replyError(c, 0, errMalformed, "unparseable envelope", "")
		return
	}
	seq := msg.uint(1)
	field, body := 0, []byte(nil)
	for f := 2; f <= 10; f++ {
		if b, ok := msg.bytes(f); ok {
			field, body = f, b
		}
	}
	req, err := parse(body)
	if err != nil || field == 0 {
		replyError(c, seq, errMalformed, "unparseable request", "")
		return
	}
	switch {
	case field == 2: // ConnectRequest
		if c.phase != 0 {
			replyError(c, seq, errInvalidState, "already connected", "")
			return
		}
		if v := req.str(1); !strings.HasPrefix(v, "1.") {
			replyError(c, seq, errInvalidProtocolVersion, "server speaks 1.2, client "+v, "")
			c.close()
			return
		}
		c.phase = 1
		var r enc
		r.str(1, c.RemoteAddr().String())
		r.str(2, "1.2")
		reply(c, seq, 2, r)
	case c.phase == 0:
		replyError(c, seq, errInvalidState, "send ConnectRequest first", "")
	case field == 3: // AuthRequest
		if c.phase != 1 {
			replyError(c, seq, errInvalidState, "already authenticated", "")
			return
		}
		if req.str(2) != "" {
			replyError(c, seq, errAuthFailed, "resuming is not supported", "")
			return
		}
		name := req.str(1)
		if name == "" || len(name) > 64 {
			replyError(c, seq, errInvalidArgument, "display_name must be 1 to 64 bytes", "")
			return
		}
		c.phase = 2
		c.player = &player{id: newID("p-"), name: name, conn: c}
		var r enc
		r.str(1, c.player.id)
		r.str(2, newID("tok-"))
		reply(c, seq, 3, r)
	case c.phase != 2:
		replyError(c, seq, errInvalidState, "send AuthRequest first", "")
	case field == 4:
		s.join(c, seq, req)
	case field == 5:
		p := c.player
		if p.session == nil || p.session.id != req.str(1) {
			replyError(c, seq, errNotInSession, "not a member of that session", req.str(1))
			return
		}
		id := p.session.id
		s.leave(p)
		var r enc
		r.str(1, id)
		reply(c, seq, 7, r)
	case field == 6:
		s.action(c, seq, req)
	case field == 7:
		s.setReady(c, seq, req)
	case field == 8:
		p := c.player
		if p.session == nil || p.session.id != req.str(1) {
			replyError(c, seq, errNotInSession, "not a member of that session", req.str(1))
			return
		}
		var r enc
		r.str(1, p.id)
		if p.session.playing {
			for _, a := range p.session.rules.LegalActions(p.session.state, p.id) {
				var spec enc
				spec.bytes(1, a.Payload)
				spec.str(2, a.Description)
				r.msg(2, spec)
			}
		}
		reply(c, seq, 8, r)
	case field == 9:
		var r enc
		for _, ss := range s.sessions {
			if ss.gameID != req.str(1) || ss.private || len(ss.members) == 0 {
				continue
			}
			var info enc
			info.str(1, ss.id)
			info.str(2, ss.gameID)
			info.uint(3, uint64(ss.status()))
			for _, m := range ss.members {
				info.msg(4, s.playerMsg(ss, m))
			}
			info.str(5, ss.members[0].id)
			info.boolean(6, s.joinable(ss))
			r.msg(1, info)
		}
		reply(c, seq, 9, r)
	case field == 10:
		p := c.player
		if p.session == nil || p.session.id != req.str(1) {
			replyError(c, seq, errNotInSession, "not a member of that session", req.str(1))
			return
		}
		var r enc
		r.str(1, p.session.id)
		for _, h := range p.session.history {
			r.msg(2, h)
		}
		reply(c, seq, 10, r)
	}
}

func (ss *session) status() int {
	if ss.playing {
		return statusInProgress
	}
	return statusWaiting
}

// seat returns the first role with room, as docs/writing-a-ruleset.md describes, or "" if none
func (s *server) seat(ss *session) game.Role {
	counts := map[game.Role]int{}
	for _, r := range ss.roles {
		counts[r]++
	}
	for _, spec := range ss.rules.Roles() {
		if spec.Max == 0 || counts[spec.Name] < spec.Max {
			return spec.Name
		}
	}
	return ""
}

func (s *server) joinable(ss *session) bool { return !ss.playing && s.seat(ss) != "" }

func (s *server) join(c *conn, seq uint64, req fields) {
	p := c.player
	gameID, sid, create, private := req.str(1), req.str(2), req.boolean(3), req.boolean(4)
	switch {
	case gameID == "":
		replyError(c, seq, errInvalidArgument, "game_id is empty", "")
		return
	case s.games[gameID] == nil:
		replyError(c, seq, errUnknownGame, fmt.Sprintf("game: unknown game %q", gameID), "")
		return
	case create && sid != "", private && !create:
		replyError(c, seq, errInvalidArgument, "bad combination of session_id, create_new and private_session", "")
		return
	case p.session != nil:
		replyError(c, seq, errAlreadyInSession, "already in a session", p.session.id)
		return
	}
	var ss *session
	if sid != "" {
		for _, x := range s.sessions {
			if x.id == sid {
				ss = x
			}
		}
		if ss == nil {
			replyError(c, seq, errSessionNotFound, "no session "+sid, sid)
			return
		}
		if !s.joinable(ss) {
			replyError(c, seq, errSessionNotJoinable, "session is full or already playing", sid)
			return
		}
		if ss.gameID != gameID {
			replyError(c, seq, errSessionNotJoinable, "session belongs to a different game", sid)
			return
		}
	} else if !create {
		for _, x := range s.sessions {
			if x.gameID == gameID && !x.private && len(x.members) > 0 && s.joinable(x) {
				ss = x
				break
			}
		}
	}
	if ss == nil {
		s.nextID++
		ss = &session{id: fmt.Sprintf("s-%d-%s", s.nextID, newID("")[:6]), gameID: gameID, rules: s.games[gameID], private: private,
			roles: map[string]game.Role{}, ready: map[string]bool{}, accepted: map[string]int{}, rejected: map[string]int{}}
		s.sessions = append(s.sessions, ss)
	}
	ss.members = append(ss.members, p)
	ss.roles[p.id] = s.seat(ss)
	p.session = ss
	ss.version++
	s.broadcast(ss, p, seq)
}

func (s *server) leave(p *player) {
	ss := p.session
	p.session = nil
	for i, m := range ss.members {
		if m == p {
			ss.members = append(ss.members[:i], ss.members[i+1:]...)
			break
		}
	}
	delete(ss.roles, p.id)
	delete(ss.ready, p.id)
	ss.version++
	if len(ss.members) == 0 {
		for i, x := range s.sessions {
			if x == ss {
				s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
			}
		}
		return
	}
	// As docs/writing-a-ruleset.md says, the game isn't told; a running match continues.
	s.broadcast(ss, nil, 0)
	if !ss.playing {
		s.maybeStart(ss)
	}
}

func (s *server) setReady(c *conn, seq uint64, req fields) {
	p := c.player
	if p.session == nil || p.session.id != req.str(1) {
		replyError(c, seq, errNotInSession, "not a member of that session", req.str(1))
		return
	}
	ss := p.session
	if ss.playing {
		replyError(c, seq, errInvalidState, "the match has started", ss.id)
		return
	}
	ss.ready[p.id] = req.boolean(2)
	ss.version++
	if ss.rules.ReadyToStart(ss.roles, ss.ready) {
		s.start(ss)
	}
	s.broadcast(ss, p, seq)
}

func (s *server) maybeStart(ss *session) {
	if ss.rules.ReadyToStart(ss.roles, ss.ready) {
		s.start(ss)
		s.broadcast(ss, nil, 0)
	}
}

func (s *server) start(ss *session) {
	order := make([]string, len(ss.members))
	for i, m := range ss.members {
		order[i] = m.id
	}
	roles := make(map[string]game.Role, len(ss.roles))
	for k, v := range ss.roles {
		roles[k] = v
	}
	ss.state = ss.rules.NewState(order, roles)
	ss.playing = true
	ss.turns = 0
	ss.accepted, ss.rejected = map[string]int{}, map[string]int{}
}

func (s *server) action(c *conn, seq uint64, req fields) {
	p := c.player
	ss := p.session
	reject := func(code int, msg string) {
		actionsRejected.Add(1)
		if ss != nil && ss.rejected != nil {
			ss.rejected[p.id]++
		}
		replyError(c, seq, code, msg, req.str(1))
	}
	if ss == nil || ss.id != req.str(1) {
		reject(errNotInSession, "not a member of that session")
		return
	}
	if v := req.uint(3); v != 0 && v != ss.version {
		reject(errStaleState, fmt.Sprintf("expected_state_version %d, current %d", v, ss.version))
		return
	}
	if !ss.playing {
		reject(errIllegalAction, "the match hasn't started")
		return
	}
	payload, _ := req.bytes(2)
	a := game.Action{ID: seq, ActorID: p.id, Payload: payload}
	active := ss.rules.ActiveTurn(ss.state)
	inTurn := false
	for _, id := range active {
		inTurn = inTurn || id == p.id
	}
	if !inTurn {
		o, ok := ss.rules.(game.OutOfTurnActions)
		if !ok || !o.AllowsOutOfTurn(ss.state, a) {
			reject(errOutOfTurn, "it is not your turn")
			return
		}
	}
	if err := ss.rules.Validate(ss.state, a); err != nil {
		if errors.Is(err, game.ErrMalformedAction) {
			reject(errMalformed, err.Error())
		} else {
			reject(errIllegalAction, err.Error())
		}
		return
	}
	ss.state = ss.rules.Apply(ss.state, a)
	ss.accepted[p.id]++
	ss.turns++
	ss.version++
	s.broadcast(ss, p, seq)
	if res, over := ss.rules.Outcome(ss.state); over {
		matchesFinished.Add(1)
		var mr enc
		mr.str(1, ss.id)
		for _, w := range res.GetWinnerIDs() {
			mr.str(2, w)
		}
		mr.boolean(3, len(res.GetWinnerIDs()) == 0)
		for _, m := range ss.members {
			if ss.roles[m.id] != game.RolePlayer {
				continue
			}
			var sum enc
			sum.str(1, m.id)
			sum.uint(2, uint64(ss.accepted[m.id]))
			sum.uint(3, uint64(ss.rejected[m.id]))
			mr.msg(4, sum)
		}
		mr.uint(5, endCompleted)
		mr.str(6, string(res.TerminationReason()))
		mr.uint(7, uint64(ss.turns))
		ss.history = append(ss.history, mr)
		for _, m := range ss.members {
			reply(m.conn, 0, 6, mr)
		}
		ss.playing = false
		ss.state = nil
		ss.ready = map[string]bool{}
		ss.version++
		s.broadcast(ss, nil, 0)
	}
}

func (s *server) playerMsg(ss *session, m *player) enc {
	var p enc
	p.str(1, m.id)
	p.str(2, m.name)
	p.boolean(3, true)
	p.boolean(4, ss.ready[m.id])
	p.str(5, string(ss.roles[m.id]))
	return p
}

// broadcast sends each member their StateUpdate; requester's copy carries in_reply_to = seq
func (s *server) broadcast(ss *session, requester *player, seq uint64) {
	for _, m := range ss.members {
		var u enc
		u.str(1, ss.id)
		u.str(2, ss.gameID)
		u.uint(3, uint64(ss.status()))
		for _, x := range ss.members {
			u.msg(4, s.playerMsg(ss, x))
		}
		u.uint(5, ss.version)
		if ss.playing {
			u.bytes(6, ss.rules.ViewFor(ss.state, m.id))
			if active := ss.rules.ActiveTurn(ss.state); len(active) == 1 {
				u.str(7, active[0])
			}
			for _, x := range ss.members {
				if ss.roles[x.id] == game.RolePlayer {
					u.str(8, x.id)
				}
			}
		}
		u.str(9, ss.members[0].id)
		if a, ok := ss.rules.(game.Annotator); ok && ss.playing {
			for k, v := range a.Annotations(ss.state, m.id) {
				var e enc
				e.str(1, k)
				e.str(2, v)
				u.msg(10, e)
			}
		}
		r := uint64(0)
		if m == requester {
			r = seq
		}
		reply(m.conn, r, 4, u)
	}
}
