/* Decoding standard and Ultimate Tic-Tac-Toe encodings, exactly as specified in each ruleset README. */
#ifndef TTT_VIEW_H
#define TTT_VIEW_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#define TTT_VIEW_LEN 11
#define TTT_CELLS 9
#define UTT_VIEW_LEN 93
#define UTT_BOARDS 9
#define UTT_CELLS 9
#define UTT_GAME_ID "ultimate_tictactoe"

typedef enum {
    TTT_RUNNING_X, /* 'X': X to move */
    TTT_RUNNING_O, /* 'O': O to move */
    TTT_X_WON,     /* 'x' */
    TTT_O_WON,     /* 'o' */
    TTT_DRAW,      /* 'D' */
    TTT_NO_RESULT  /* '-' */
} ttt_status;

typedef struct {
    char cells[TTT_CELLS]; /* 'X', 'O' or '.' */
    ttt_status status;
    char me; /* 'X', 'O' or 'S' (spectator) */
} ttt_view;

typedef struct {
    char cells[UTT_BOARDS * UTT_CELLS];
    char local_status[UTT_BOARDS]; /* '.', 'X', 'O' or 'D' */
    int next_board; /* 0..8, or -1 when any unfinished board is available */
    ttt_status status;
    char me; /* 'X', 'O' or 'S' (spectator) */
} ultimate_view;

/* Decodes an encoded_view. Returns false, leaving *out untouched, unless it is exactly 11 valid bytes
 * (an empty view, sent while the lobby waits, is not a board). */
bool ttt_view_decode(const uint8_t *data, size_t len, ttt_view *out);

bool ttt_view_running(const ttt_view *v);
/* The mark to move, 'X' or 'O', or 0 once the game is over */
char ttt_view_to_move(const ttt_view *v);
/* True when the recipient is a player and it is their move */
bool ttt_view_my_turn(const ttt_view *v);
/* The winning mark, 'X' or 'O', or 0 */
char ttt_view_winner(const ttt_view *v);

/* encoded_action for placing a mark in cell 0..8: one ASCII digit. Returns 1, or 0 for a bad cell. */
size_t ttt_action_move(int cell, uint8_t out[1]);
/* encoded_action for resigning: the 6 bytes "resign" (no terminator) */
extern const uint8_t TTT_ACTION_RESIGN[6];

/* Parses an annotation like winning_line "0,4,8" into three cells. Returns false if malformed. */
bool ttt_parse_line(const char *s, int out[3]);

bool ultimate_view_decode(const uint8_t *data, size_t len, ultimate_view *out);
bool ultimate_view_running(const ultimate_view *v);
char ultimate_view_to_move(const ultimate_view *v);
bool ultimate_view_my_turn(const ultimate_view *v);
char ultimate_view_winner(const ultimate_view *v);
/* Encodes board and cell (both 0..8) as two ASCII digits. */
size_t ultimate_action_move(int board, int cell, uint8_t out[2]);

#endif
