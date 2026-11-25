/* Decoding encoded_view and building encoded_action, against the examples in
 * rulesets/tictactoe/README.md. */
#include <string.h>

#include "helpers.h"
#include "view.h"

static ttt_view decode_ok(const char *s) {
    ttt_view v;
    memset(&v, 0, sizeof v);
    assert_true(ttt_view_decode((const uint8_t *)s, strlen(s), &v));
    return v;
}

static void test_readme_examples(void **s) {
    (void)s;
    ttt_view v = decode_ok(".........XX"); /* start, sent to X */
    assert_memory_equal(v.cells, ".........", 9);
    assert_int_equal(v.status, TTT_RUNNING_X);
    assert_int_equal(v.me, 'X');
    assert_true(ttt_view_my_turn(&v));

    v = decode_ok(".........XO"); /* start, sent to O */
    assert_false(ttt_view_my_turn(&v));
    assert_int_equal(ttt_view_to_move(&v), 'X');

    v = decode_ok("....X....OO"); /* X took the center, O to move, sent to O */
    assert_int_equal(v.cells[4], 'X');
    assert_true(ttt_view_my_turn(&v));

    v = decode_ok("O...X....XS"); /* sent to a spectator */
    assert_int_equal(v.me, 'S');
    assert_false(ttt_view_my_turn(&v));
    assert_true(ttt_view_running(&v));

    v = decode_ok("XXXOO....xO"); /* X won */
    assert_int_equal(v.status, TTT_X_WON);
    assert_false(ttt_view_running(&v));
    assert_int_equal(ttt_view_winner(&v), 'X');
    assert_int_equal(ttt_view_to_move(&v), 0);

    v = decode_ok("....X....oX"); /* X resigned */
    assert_int_equal(ttt_view_winner(&v), 'O');

    v = decode_ok("XOXXOOOXXDS"); /* draw */
    assert_int_equal(v.status, TTT_DRAW);
    assert_int_equal(ttt_view_winner(&v), 0);

    v = decode_ok(".........-X");
    assert_int_equal(v.status, TTT_NO_RESULT);
}

static void test_rejects_bad_views(void **s) {
    (void)s;
    const char *bad[] = {
        "",              /* the waiting lobby's empty view */
        ".........X",    /* too short */
        ".........XXX",  /* too long */
        "........ XX",   /* bad cell */
        "x........XX",   /* lowercase cell */
        ".........ZX",   /* bad status */
        ".........XQ",   /* bad "me" */
    };
    for (size_t i = 0; i < sizeof bad / sizeof bad[0]; i++) {
        ttt_view v;
        memset(&v, 0x5a, sizeof v);
        ttt_view before = v;
        assert_false(ttt_view_decode((const uint8_t *)bad[i], strlen(bad[i]), &v));
        assert_memory_equal(&v, &before, sizeof v); /* untouched */
    }
    ttt_view v;
    assert_false(ttt_view_decode(NULL, 11, &v));
    const uint8_t nul[11] = {'.', '.', '.', '.', 0, '.', '.', '.', '.', 'X', 'X'};
    assert_false(ttt_view_decode(nul, 11, &v));
}

static void test_actions(void **s) {
    (void)s;
    uint8_t b[1];
    for (int c = 0; c < 9; c++) {
        assert_int_equal(ttt_action_move(c, b), 1);
        assert_int_equal(b[0], '0' + c);
    }
    assert_int_equal(ttt_action_move(-1, b), 0);
    assert_int_equal(ttt_action_move(9, b), 0);
    assert_memory_equal(TTT_ACTION_RESIGN, "resign", 6);
    assert_int_equal(sizeof TTT_ACTION_RESIGN, 6); /* no terminator */
}

static void test_parse_line(void **s) {
    (void)s;
    int l[3];
    assert_true(ttt_parse_line("0,4,8", l));
    assert_int_equal(l[0], 0);
    assert_int_equal(l[1], 4);
    assert_int_equal(l[2], 8);
    assert_true(ttt_parse_line("2,4,6", l));
    assert_false(ttt_parse_line("", l));
    assert_false(ttt_parse_line("0,4", l));
    assert_false(ttt_parse_line("0,4,9", l));
    assert_false(ttt_parse_line("0,4,8,", l));
    assert_false(ttt_parse_line("0;4;8", l));
    assert_false(ttt_parse_line(NULL, l));
}

static void test_ultimate_view_and_actions(void **s) {
    (void)s;
    uint8_t bytes[UTT_VIEW_LEN];
    memset(bytes, '.', 91);
    bytes[81 + 5] = 'D';
    bytes[90] = '4';
    bytes[91] = 'O';
    bytes[92] = 'O';
    ultimate_view v;
    assert_true(ultimate_view_decode(bytes, sizeof bytes, &v));
    assert_int_equal(v.cells[0], '.');
    assert_int_equal(v.local_status[5], 'D');
    assert_int_equal(v.next_board, 4);
    assert_int_equal(v.status, TTT_RUNNING_O);
    assert_int_equal(v.me, 'O');
    assert_true(ultimate_view_my_turn(&v));
    assert_int_equal(ultimate_view_to_move(&v), 'O');

    bytes[90] = '.';
    bytes[91] = 'x';
    assert_true(ultimate_view_decode(bytes, sizeof bytes, &v));
    assert_false(ultimate_view_running(&v));
    assert_int_equal(ultimate_view_winner(&v), 'X');
    bytes[91] = 'D';
    for (int board = 0; board < UTT_BOARDS; board++) memcpy(bytes + board * UTT_CELLS, "XOXXOOOXX", UTT_CELLS);
    memset(bytes + 81, 'D', UTT_BOARDS);
    bytes[90] = '.';
    assert_true(ultimate_view_decode(bytes, sizeof bytes, &v));
    assert_int_equal(v.status, TTT_DRAW);

    uint8_t action[2];
    assert_int_equal(ultimate_action_move(4, 8, action), 2);
    assert_memory_equal(action, "48", 2);
    assert_int_equal(ultimate_action_move(-1, 0, action), 0);
    assert_int_equal(ultimate_action_move(9, 0, action), 0);
    assert_int_equal(ultimate_action_move(0, 9, action), 0);
}

static void test_rejects_bad_ultimate_views(void **s) {
    (void)s;
    uint8_t bytes[UTT_VIEW_LEN];
    memset(bytes, '.', 91);
    bytes[91] = 'X';
    bytes[92] = 'S';
    ultimate_view v;
    memset(&v, 0x5a, sizeof v);
    ultimate_view before = v;
    assert_false(ultimate_view_decode(bytes, sizeof bytes - 1, &v));
    assert_memory_equal(&v, &before, sizeof v);
    bytes[17] = 'Z';
    assert_false(ultimate_view_decode(bytes, sizeof bytes, &v));
    bytes[17] = '.';
    bytes[81] = 'z';
    assert_false(ultimate_view_decode(bytes, sizeof bytes, &v));
    bytes[81] = '.';
    bytes[90] = '9';
    assert_false(ultimate_view_decode(bytes, sizeof bytes, &v));
    bytes[90] = '.';
    bytes[92] = '?';
    assert_false(ultimate_view_decode(bytes, sizeof bytes, &v));
    assert_memory_equal(&v, &before, sizeof v);
    bytes[92] = 'X';
    bytes[81] = 'D';
    bytes[90] = '0';
    assert_false(ultimate_view_decode(bytes, sizeof bytes, &v));
    bytes[81] = '.';
    bytes[91] = 'x';
    assert_false(ultimate_view_decode(bytes, sizeof bytes, &v));
    assert_memory_equal(&v, &before, sizeof v);
}

int main(void) {
    const struct CMUnitTest tests[] = {
        cmocka_unit_test(test_readme_examples),
        cmocka_unit_test(test_rejects_bad_views),
        cmocka_unit_test(test_actions),
        cmocka_unit_test(test_parse_line),
        cmocka_unit_test(test_ultimate_view_and_actions),
        cmocka_unit_test(test_rejects_bad_ultimate_views),
    };
    return cmocka_run_group_tests(tests, NULL, NULL);
}
