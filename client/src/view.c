#include "view.h"

const uint8_t TTT_ACTION_RESIGN[6] = {'r', 'e', 's', 'i', 'g', 'n'};

bool ttt_view_decode(const uint8_t *data, size_t len, ttt_view *out) {
    if (data == NULL || len != TTT_VIEW_LEN) return false;
    ttt_view v;
    for (int i = 0; i < TTT_CELLS; i++) {
        char c = (char)data[i];
        if (c != 'X' && c != 'O' && c != '.') return false;
        v.cells[i] = c;
    }
    switch ((char)data[9]) {
    case 'X': v.status = TTT_RUNNING_X; break;
    case 'O': v.status = TTT_RUNNING_O; break;
    case 'x': v.status = TTT_X_WON; break;
    case 'o': v.status = TTT_O_WON; break;
    case 'D': v.status = TTT_DRAW; break;
    case '-': v.status = TTT_NO_RESULT; break;
    default: return false;
    }
    char me = (char)data[10];
    if (me != 'X' && me != 'O' && me != 'S') return false;
    v.me = me;
    *out = v;
    return true;
}

bool ultimate_view_decode(const uint8_t *data, size_t len, ultimate_view *out) {
    if (data == NULL || len != UTT_VIEW_LEN) return false;
    ultimate_view v;
    for (int i = 0; i < UTT_BOARDS * UTT_CELLS; i++) {
        char c = (char)data[i];
        if (c != 'X' && c != 'O' && c != '.') return false;
        v.cells[i] = c;
    }
    for (int i = 0; i < UTT_BOARDS; i++) {
        char c = (char)data[81 + i];
        if (c != '.' && c != 'X' && c != 'O' && c != 'D') return false;
        v.local_status[i] = c;
    }
    char next = (char)data[90];
    if (next == '.') v.next_board = -1;
    else if (next >= '0' && next <= '8') v.next_board = next - '0';
    else return false;
    if (v.next_board >= 0 && v.local_status[v.next_board] != '.') return false;
    switch ((char)data[91]) {
    case 'X': v.status = TTT_RUNNING_X; break;
    case 'O': v.status = TTT_RUNNING_O; break;
    case 'x': v.status = TTT_X_WON; break;
    case 'o': v.status = TTT_O_WON; break;
    case 'D': v.status = TTT_DRAW; break;
    case '-': v.status = TTT_NO_RESULT; break;
    default: return false;
    }
    char me = (char)data[92];
    if (me != 'X' && me != 'O' && me != 'S') return false;
    v.me = me;
    bool has_open_board = false;
    for (int i = 0; i < UTT_BOARDS; i++) has_open_board = has_open_board || v.local_status[i] == '.';
    bool running = ultimate_view_running(&v);
    if ((running && !has_open_board) || (!running && v.next_board != -1) ||
        (v.status == TTT_DRAW && has_open_board)) return false;
    *out = v;
    return true;
}

bool ttt_view_running(const ttt_view *v) { return v->status == TTT_RUNNING_X || v->status == TTT_RUNNING_O; }

char ttt_view_to_move(const ttt_view *v) {
    if (v->status == TTT_RUNNING_X) return 'X';
    if (v->status == TTT_RUNNING_O) return 'O';
    return 0;
}

bool ttt_view_my_turn(const ttt_view *v) { return ttt_view_running(v) && ttt_view_to_move(v) == v->me; }

char ttt_view_winner(const ttt_view *v) {
    if (v->status == TTT_X_WON) return 'X';
    if (v->status == TTT_O_WON) return 'O';
    return 0;
}

size_t ttt_action_move(int cell, uint8_t out[1]) {
    if (cell < 0 || cell >= TTT_CELLS) return 0;
    out[0] = (uint8_t)('0' + cell);
    return 1;
}

bool ttt_parse_line(const char *s, int out[3]) {
    /* exactly "d,d,d" with digits 0..8 */
    if (s == NULL) return false;
    for (int i = 0; i < 3; i++) {
        if (s[0] < '0' || s[0] > '8') return false;
        out[i] = s[0] - '0';
        s++;
        if (i < 2) {
            if (*s != ',') return false;
            s++;
        }
    }
    return *s == '\0';
}

bool ultimate_view_running(const ultimate_view *v) {
    return v->status == TTT_RUNNING_X || v->status == TTT_RUNNING_O;
}

char ultimate_view_to_move(const ultimate_view *v) {
    if (v->status == TTT_RUNNING_X) return 'X';
    if (v->status == TTT_RUNNING_O) return 'O';
    return 0;
}

bool ultimate_view_my_turn(const ultimate_view *v) {
    return ultimate_view_running(v) && ultimate_view_to_move(v) == v->me;
}

char ultimate_view_winner(const ultimate_view *v) {
    if (v->status == TTT_X_WON) return 'X';
    if (v->status == TTT_O_WON) return 'O';
    return 0;
}

size_t ultimate_action_move(int board, int cell, uint8_t out[2]) {
    if (board < 0 || board >= UTT_BOARDS || cell < 0 || cell >= UTT_CELLS) return 0;
    out[0] = (uint8_t)('0' + board);
    out[1] = (uint8_t)('0' + cell);
    return 2;
}
