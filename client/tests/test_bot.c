/* The bot: takes wins, blocks losses, and never loses with perfect play */
#include <string.h>

#include "bot.h"
#include "helpers.h"

static char winner(const char *b) {
    static const int L[8][3] = {{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {0, 3, 6}, {1, 4, 7}, {2, 5, 8}, {0, 4, 8}, {2, 4, 6}};
    for (int i = 0; i < 8; i++)
        if (b[L[i][0]] != '.' && b[L[i][0]] == b[L[i][1]] && b[L[i][1]] == b[L[i][2]]) return b[L[i][0]];
    return 0;
}

static void test_takes_win_and_blocks(void **s) {
    (void)s;
    unsigned seed = 1;
    assert_int_equal(bot_choose("XX.OO....", 'X', BOT_PERFECT, &seed), 2); /* win */
    assert_int_equal(bot_choose("XX.OO....", 'O', BOT_PERFECT, &seed), 5); /* win rather than block */
    assert_int_equal(bot_choose("XX..O....", 'O', BOT_PERFECT, &seed), 2); /* block */
    assert_int_equal(bot_choose("XOXXOOOXX", 'X', BOT_PERFECT, &seed), -1); /* full */
}

/* Every game against every possible opponent move sequence: perfect play never loses */
static int losses;
static void explore(char *b, char turn, char bot_mark, unsigned *seed) {
    char w = winner(b);
    if (w) {
        if (w != bot_mark) losses++;
        return;
    }
    if (!memchr(b, '.', 9)) return;
    char next = turn == 'X' ? 'O' : 'X';
    if (turn == bot_mark) {
        int c = bot_choose(b, bot_mark, BOT_PERFECT, seed);
        assert_true(c >= 0 && b[c] == '.');
        b[c] = turn;
        explore(b, next, bot_mark, seed);
        b[c] = '.';
    } else {
        for (int i = 0; i < 9; i++) {
            if (b[i] != '.') continue;
            b[i] = turn;
            explore(b, next, bot_mark, seed);
            b[i] = '.';
        }
    }
}

static void test_never_loses(void **s) {
    (void)s;
    unsigned seed = 7;
    char b[10] = ".........";
    losses = 0;
    explore(b, 'X', 'X', &seed);
    explore(b, 'X', 'O', &seed);
    assert_int_equal(losses, 0);
}

static void test_casual_plays_legal_moves(void **s) {
    (void)s;
    unsigned seed = 3;
    for (int i = 0; i < 200; i++) {
        int c = bot_choose("X.O.X.O..", 'X', BOT_CASUAL, &seed);
        assert_true(c == 1 || c == 3 || c == 5 || c == 7 || c == 8);
    }
}

static void test_ultimate_bot_respects_next_board(void **s) {
    (void)s;
    ultimate_view view;
    memset(&view, 0, sizeof view);
    memset(view.cells, '.', sizeof view.cells);
    memset(view.local_status, '.', sizeof view.local_status);
    view.status = TTT_RUNNING_X;
    view.me = 'X';
    view.next_board = 4;
    view.cells[4 * UTT_CELLS + 2] = 'O';
    unsigned seed = 12;
    int board, cell;
    for (int i = 0; i < 50; i++) {
        assert_true(bot_choose_ultimate(&view, &seed, &board, &cell));
        assert_int_equal(board, 4);
        assert_true(cell >= 0 && cell < UTT_CELLS && view.cells[4 * UTT_CELLS + cell] == '.');
    }
    view.next_board = -1;
    view.local_status[4] = 'D';
    assert_true(bot_choose_ultimate(&view, &seed, &board, &cell));
    assert_true(board != 4);
    view.status = TTT_DRAW;
    assert_false(bot_choose_ultimate(&view, &seed, &board, &cell));
}

int main(void) {
    const struct CMUnitTest tests[] = {cmocka_unit_test(test_takes_win_and_blocks), cmocka_unit_test(test_never_loses),
                                       cmocka_unit_test(test_casual_plays_legal_moves),
                                       cmocka_unit_test(test_ultimate_bot_respects_next_board)};
    return cmocka_run_group_tests(tests, NULL, NULL);
}
