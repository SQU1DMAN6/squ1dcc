/*
 * terminal_win_sqx — Windows Terminal Control SQX Module (v1.9)
 *
 * Provides low-level Windows console API access for SQU1DLang.
 * Uses the SQX structured return contract and SQX Runtime Library.
 *
 * Features:
 *   - Output: write, writeln, clear, write_at, move, scroll, set_title, set_size
 *   - Styling: set_fg, set_bg, reset_style
 *   - Screen: enter_alt_screen, exit_alt_screen, show_cursor
 *   - Input: read_key, read_keys, poll_event, flush_input
 *   - Raw mode: enable_raw_mode, disable_raw_mode
 *   - Info: get_size, cursor_pos
 *   - Session mode: --session (persistent JSON protocol)
 *
 * BUILD (on Windows with MSVC/MinGW):
 *   cl /O2 /I"..\..\src\include" terminal_win.sqx main.c
 *   gcc -O2 -I"../../src/include" -o terminal_win.sqx main.c
 *
 * USAGE (from SQU1DLang):
 *   var term = pkg.load_sqx("path/to/terminal_win.sqx")
 *   term.write("Hello, console!")
 *   term.enable_raw_mode()
 *   var evt = term.read_key()
 *   term.disable_raw_mode()
 */

#include "sqx_runtime.h"

/* ---- Windows Console State ---- */
static HANDLE g_hStdout = NULL;
static HANDLE g_hStdin  = NULL;
static HANDLE g_hStderr = NULL;
static int    g_raw_mode = 0;
static DWORD  g_original_console_mode = 0;
static int    g_initialized = 0;

/* Key name lookup for virtual key codes */
typedef struct { WORD vk; const char *name; } KeyName;

static KeyName g_key_names[] = {
    {VK_BACK,      "backspace"},
    {VK_TAB,       "tab"},
    {VK_CLEAR,     "clear"},
    {VK_RETURN,    "enter"},
    {VK_SHIFT,     "shift"},
    {VK_CONTROL,   "ctrl"},
    {VK_MENU,      "alt"},
    {VK_PAUSE,     "pause"},
    {VK_CAPITAL,   "caps_lock"},
    {VK_ESCAPE,    "escape"},
    {VK_SPACE,     "space"},
    {VK_PRIOR,     "page_up"},
    {VK_NEXT,      "page_down"},
    {VK_END,       "end"},
    {VK_HOME,      "home"},
    {VK_LEFT,      "left"},
    {VK_UP,        "up"},
    {VK_RIGHT,     "right"},
    {VK_DOWN,      "down"},
    {VK_SELECT,    "select"},
    {VK_PRINT,     "print"},
    {VK_EXECUTE,   "execute"},
    {VK_SNAPSHOT,  "print_screen"},
    {VK_INSERT,    "insert"},
    {VK_DELETE,    "delete"},
    {VK_HELP,      "help"},
    {VK_LWIN,      "lwin"},
    {VK_RWIN,      "rwin"},
    {VK_APPS,      "apps"},
    {VK_SLEEP,     "sleep"},
    {VK_NUMPAD0,   "numpad_0"},
    {VK_NUMPAD1,   "numpad_1"},
    {VK_NUMPAD2,   "numpad_2"},
    {VK_NUMPAD3,   "numpad_3"},
    {VK_NUMPAD4,   "numpad_4"},
    {VK_NUMPAD5,   "numpad_5"},
    {VK_NUMPAD6,   "numpad_6"},
    {VK_NUMPAD7,   "numpad_7"},
    {VK_NUMPAD8,   "numpad_8"},
    {VK_NUMPAD9,   "numpad_9"},
    {VK_MULTIPLY,  "numpad_multiply"},
    {VK_ADD,       "numpad_add"},
    {VK_SEPARATOR, "numpad_separator"},
    {VK_SUBTRACT,  "numpad_subtract"},
    {VK_DECIMAL,   "numpad_decimal"},
    {VK_DIVIDE,    "numpad_divide"},
    {VK_F1,        "f1"},
    {VK_F2,        "f2"},
    {VK_F3,        "f3"},
    {VK_F4,        "f4"},
    {VK_F5,        "f5"},
    {VK_F6,        "f6"},
    {VK_F7,        "f7"},
    {VK_F8,        "f8"},
    {VK_F9,        "f9"},
    {VK_F10,       "f10"},
    {VK_F11,       "f11"},
    {VK_F12,       "f12"},
    {VK_NUMLOCK,   "num_lock"},
    {VK_SCROLL,    "scroll_lock"},
    {VK_LSHIFT,    "lshift"},
    {VK_RSHIFT,    "rshift"},
    {VK_LCONTROL,  "lctrl"},
    {VK_RCONTROL,  "rctrl"},
    {VK_LMENU,     "lalt"},
    {VK_RMENU,     "ralt"},
    {0, NULL}
};

/* Convert virtual key code to name string. Returns a pointer to static data. */
static const char* vk_to_name(WORD vk) {
    /* Printable ASCII */
    if (vk >= '0' && vk <= '9') {
        static char buf[2];
        buf[0] = (char)vk; buf[1] = '\0';
        return buf;
    }
    if (vk >= 'A' && vk <= 'Z') {
        static char buf[2];
        buf[0] = (char)(vk - 'A' + 'a');
        buf[1] = '\0';
        return buf;
    }
    /* Named keys */
    for (int i = 0; g_key_names[i].name; i++) {
        if (g_key_names[i].vk == vk) return g_key_names[i].name;
    }
    /* OEM keys */
    if (vk >= VK_OEM_1 && vk <= VK_OEM_102) {
        static char buf[4];
        switch (vk) {
            case VK_OEM_1:      return ";";
            case VK_OEM_PLUS:   return "=";
            case VK_OEM_COMMA:  return ",";
            case VK_OEM_MINUS:  return "-";
            case VK_OEM_PERIOD: return ".";
            case VK_OEM_2:      return "/";
            case VK_OEM_3:      return "`";
            case VK_OEM_4:      return "[";
            case VK_OEM_5:      return "\\";
            case VK_OEM_6:      return "]";
            case VK_OEM_7:      return "'";
            case VK_OEM_8:      return "\xc2\xa7";
            case VK_OEM_102:    return "\\";
        }
    }
    static char buf[16];
    snprintf(buf, sizeof(buf), "vk_0x%02x", vk);
    return buf;
}

/* ---- Initialization ---- */
static void ensure_handles(void) {
    if (g_initialized) return;
    g_hStdout = GetStdHandle(STD_OUTPUT_HANDLE);
    g_hStdin  = GetStdHandle(STD_INPUT_HANDLE);
    g_hStderr = GetStdHandle(STD_ERROR_HANDLE);
    g_initialized = 1;
}

/* ---- JSON helpers for event serialization ---- */

/* Append modifier JSON array to buffer */
static void append_modifiers(char *buf, size_t buf_size, DWORD ctrl_key_state) {
    char *p = buf;
    *p++ = '[';
    int first = 1;
    if (ctrl_key_state & SHIFT_PRESSED) {
        p += snprintf(p, buf_size - (p - buf), "%s\"shift\"", first ? "" : ","); first = 0;
    }
    if (ctrl_key_state & (LEFT_CTRL_PRESSED | RIGHT_CTRL_PRESSED)) {
        p += snprintf(p, buf_size - (p - buf), "%s\"ctrl\"", first ? "" : ","); first = 0;
    }
    if (ctrl_key_state & (LEFT_ALT_PRESSED | RIGHT_ALT_PRESSED)) {
        p += snprintf(p, buf_size - (p - buf), "%s\"alt\"", first ? "" : ","); first = 0;
    }
    *p++ = ']';
    *p = '\0';
}

/* Serialize a KEY_EVENT_RECORD to JSON body (without outer braces) */
static void key_event_to_json(const KEY_EVENT_RECORD *ker, char *buf, size_t buf_size) {
    const char *key_name = vk_to_name(ker->wVirtualKeyCode);
    char uchar[5] = {0};
    if (ker->uChar.AsciiChar != 0) {
        uchar[0] = ker->uChar.AsciiChar;
    }
    char mod_json[64];
    append_modifiers(mod_json, sizeof(mod_json), ker->dwControlKeyState);

    snprintf(buf, buf_size,
        "\"type\":\"key\","
        "\"key\":\"%s\","
        "\"char\":\"%s\","
        "\"pressed\":%s,"
        "\"repeat_count\":%d,"
        "\"vk_code\":%u,"
        "\"modifiers\":%s,"
        "\"ctrl\":%s,\"alt\":%s,\"shift\":%s",
        key_name, uchar,
        ker->bKeyDown ? "true" : "false",
        ker->wRepeatCount,
        ker->wVirtualKeyCode,
        mod_json,
        (ker->dwControlKeyState & (LEFT_CTRL_PRESSED | RIGHT_CTRL_PRESSED)) ? "true" : "false",
        (ker->dwControlKeyState & (LEFT_ALT_PRESSED | RIGHT_ALT_PRESSED)) ? "true" : "false",
        (ker->dwControlKeyState & SHIFT_PRESSED) ? "true" : "false");
}

/* Serialize a MOUSE_EVENT_RECORD to JSON body */
static void mouse_event_to_json(const MOUSE_EVENT_RECORD *mer, char *buf, size_t buf_size) {
    const char *button = "none";
    if (mer->dwButtonState & FROM_LEFT_1ST_BUTTON_PRESSED) button = "left";
    else if (mer->dwButtonState & RIGHTMOST_BUTTON_PRESSED) button = "right";
    else if (mer->dwButtonState & FROM_LEFT_2ND_BUTTON_PRESSED) button = "middle";

    const char *event_type = "mouse_move";
    if (mer->dwEventFlags & DOUBLE_CLICK) event_type = "mouse_double_click";
    else if (mer->dwEventFlags & MOUSE_WHEELED) event_type = "mouse_wheel";
    else if (mer->dwEventFlags & MOUSE_HWHEELED) event_type = "mouse_hwheel";

    char mod_json[64];
    append_modifiers(mod_json, sizeof(mod_json), mer->dwControlKeyState);

    char wheel[16] = "none";
    if (mer->dwEventFlags & MOUSE_WHEELED) {
        snprintf(wheel, sizeof(wheel), "%s", (short)mer->dwButtonState > 0 ? "up" : "down");
    }

    snprintf(buf, buf_size,
        "\"type\":\"mouse\","
        "\"event\":\"%s\","
        "\"x\":%d,\"y\":%d,"
        "\"button\":\"%s\","
        "\"wheel\":\"%s\","
        "\"modifiers\":%s",
        event_type, mer->dwMousePosition.X, mer->dwMousePosition.Y,
        button, wheel, mod_json);
}

/* Serialize a WINDOW_BUFFER_SIZE_RECORD to JSON body */
static void resize_event_to_json(const WINDOW_BUFFER_SIZE_RECORD *wsr, char *buf, size_t buf_size) {
    snprintf(buf, buf_size,
        "\"type\":\"resize\",\"width\":%d,\"height\":%d",
        wsr->dwSize.X, wsr->dwSize.Y);
}

/* ---- Output Functions ---- */

static void cmd_write(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("write requires 1 argument (text)"); return; }
    ensure_handles();
    DWORD written = 0;
    const char *text = argv[0];
    size_t len = strlen(text);
    if (!WriteConsoleA(g_hStdout, text, (DWORD)len, &written, NULL)) {
        fprintf(stdout, "%s", text);
        fflush(stdout);
    }
    sqx_write_bool(1);
}

static void cmd_writeln(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("writeln requires 1 argument (text)"); return; }
    ensure_handles();
    DWORD written = 0;
    const char *text = argv[0];
    size_t len = strlen(text);
    if (!WriteConsoleA(g_hStdout, text, (DWORD)len, &written, NULL)) {
        fprintf(stdout, "%s\n", text);
        fflush(stdout);
    } else {
        WriteConsoleA(g_hStdout, "\n", 1, &written, NULL);
    }
    sqx_write_bool(1);
}

static void cmd_clear(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    DWORD written = 0;
    const char *ansi_clear = "\x1b[2J\x1b[H";
    if (!WriteConsoleA(g_hStdout, ansi_clear, (DWORD)strlen(ansi_clear), &written, NULL)) {
        system("cls");
    }
    sqx_write_bool(1);
}

static void cmd_move(int argc, char **argv) {
    if (argc < 2) { sqx_write_error("move requires 2 arguments (x, y)"); return; }
    int x = atoi(argv[0]);
    int y = atoi(argv[1]);
    ensure_handles();
    char buf[32];
    snprintf(buf, sizeof(buf), "\x1b[%d;%dH", y + 1, x + 1);
    DWORD written = 0;
    WriteConsoleA(g_hStdout, buf, (DWORD)strlen(buf), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_write_at(int argc, char **argv) {
    if (argc < 3) { sqx_write_error("write_at requires 3 arguments (x, y, text)"); return; }
    int x = atoi(argv[0]);
    int y = atoi(argv[1]);
    const char *text = argv[2];
    ensure_handles();
    COORD pos;
    pos.X = (SHORT)x;
    pos.Y = (SHORT)y;
    SetConsoleCursorPosition(g_hStdout, pos);
    DWORD written = 0;
    WriteConsoleA(g_hStdout, text, (DWORD)strlen(text), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_set_fg(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("set_fg requires 1 argument (color)"); return; }
    ensure_handles();
    int color = atoi(argv[0]);
    char buf[16];
    snprintf(buf, sizeof(buf), "\x1b[38;5;%dm", color);
    DWORD written = 0;
    WriteConsoleA(g_hStdout, buf, (DWORD)strlen(buf), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_set_bg(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("set_bg requires 1 argument (color)"); return; }
    ensure_handles();
    int color = atoi(argv[0]);
    char buf[16];
    snprintf(buf, sizeof(buf), "\x1b[48;5;%dm", color);
    DWORD written = 0;
    WriteConsoleA(g_hStdout, buf, (DWORD)strlen(buf), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_reset_style(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    const char *ansi_reset = "\x1b[0m";
    DWORD written = 0;
    WriteConsoleA(g_hStdout, ansi_reset, (DWORD)strlen(ansi_reset), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_get_size(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    CONSOLE_SCREEN_BUFFER_INFO csbi;
    if (!GetConsoleScreenBufferInfo(g_hStdout, &csbi)) {
        sqx_write_error("failed to get console buffer info");
        return;
    }
    char buf[64];
    snprintf(buf, sizeof(buf),
        "\"width\":%d,\"height\":%d",
        csbi.srWindow.Right - csbi.srWindow.Left + 1,
        csbi.srWindow.Bottom - csbi.srWindow.Top + 1);
    sqx_write_json_object(buf);
}

static void cmd_cursor_pos(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    CONSOLE_SCREEN_BUFFER_INFO csbi;
    if (!GetConsoleScreenBufferInfo(g_hStdout, &csbi)) {
        sqx_write_error("failed to get cursor position");
        return;
    }
    char buf[64];
    snprintf(buf, sizeof(buf), "\"x\":%d,\"y\":%d",
        csbi.dwCursorPosition.X, csbi.dwCursorPosition.Y);
    sqx_write_json_object(buf);
}

static void cmd_enter_alt_screen(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    const char *alt = "\x1b[?1049h";
    DWORD written = 0;
    WriteConsoleA(g_hStdout, alt, (DWORD)strlen(alt), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_exit_alt_screen(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    const char *alt = "\x1b[?1049l";
    DWORD written = 0;
    WriteConsoleA(g_hStdout, alt, (DWORD)strlen(alt), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_show_cursor(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("show_cursor requires 1 argument (show)"); return; }
    int show = (strcmp(argv[0], "true") == 0 || strcmp(argv[0], "1") == 0);
    ensure_handles();
    const char *seq = show ? "\x1b[?25h" : "\x1b[?25l";
    DWORD written = 0;
    WriteConsoleA(g_hStdout, seq, (DWORD)strlen(seq), &written, NULL);
    sqx_write_bool(1);
}

static void cmd_scroll(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("scroll requires 1 argument (rows)"); return; }
    int rows = atoi(argv[0]);
    ensure_handles();
    CONSOLE_SCREEN_BUFFER_INFO csbi;
    if (!GetConsoleScreenBufferInfo(g_hStdout, &csbi)) {
        sqx_write_error("failed to get console info");
        return;
    }
    SMALL_RECT scrollRect;
    COORD destOrigin;
    CHAR_INFO fill;
    if (rows > 0) {
        scrollRect.Top = 0;
        scrollRect.Bottom = csbi.dwSize.Y - 1;
        scrollRect.Left = 0;
        scrollRect.Right = csbi.dwSize.X - 1;
        destOrigin.X = 0;
        destOrigin.Y = -rows;
    } else {
        scrollRect.Top = -rows;
        scrollRect.Bottom = csbi.dwSize.Y - 1;
        scrollRect.Left = 0;
        scrollRect.Right = csbi.dwSize.X - 1;
        destOrigin.X = 0;
        destOrigin.Y = 0;
    }
    fill.Char.AsciiChar = ' ';
    fill.Attributes = csbi.wAttributes;
    ScrollConsoleScreenBuffer(g_hStdout, &scrollRect, NULL, destOrigin, &fill);
    sqx_write_bool(1);
}

static void cmd_set_title(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("set_title requires 1 argument (title)"); return; }
    if (!SetConsoleTitleA(argv[0])) {
        sqx_write_error("failed to set console title");
        return;
    }
    sqx_write_bool(1);
}

static void cmd_set_size(int argc, char **argv) {
    if (argc < 2) { sqx_write_error("set_size requires 2 arguments (width, height)"); return; }
    int width = atoi(argv[0]);
    int height = atoi(argv[1]);
    ensure_handles();
    SMALL_RECT window;
    window.Top = 0; window.Left = 0;
    window.Bottom = height - 1; window.Right = width - 1;
    COORD size;
    size.X = (SHORT)width; size.Y = (SHORT)height;
    SetConsoleScreenBufferSize(g_hStdout, size);
    SetConsoleWindowInfo(g_hStdout, TRUE, &window);
    sqx_write_bool(1);
}

/* ---- Raw Mode Functions ---- */

static void cmd_enable_raw_mode(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    if (g_raw_mode) { sqx_write_bool(1); return; }
    if (!GetConsoleMode(g_hStdin, &g_original_console_mode)) {
        sqx_write_error("failed to get console mode");
        return;
    }
    DWORD new_mode = g_original_console_mode;
    new_mode &= ~(ENABLE_ECHO_INPUT | ENABLE_LINE_INPUT | ENABLE_PROCESSED_INPUT);
    new_mode |= ENABLE_VIRTUAL_TERMINAL_INPUT | ENABLE_MOUSE_INPUT | ENABLE_WINDOW_INPUT;
    if (!SetConsoleMode(g_hStdin, new_mode)) {
        sqx_write_error("failed to set raw console mode");
        return;
    }
    DWORD out_mode = 0;
    if (GetConsoleMode(g_hStdout, &out_mode)) {
        out_mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING;
        SetConsoleMode(g_hStdout, out_mode);
    }
    g_raw_mode = 1;
    sqx_write_bool(1);
}

static void cmd_disable_raw_mode(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    if (!g_raw_mode) { sqx_write_bool(1); return; }
    SetConsoleMode(g_hStdin, g_original_console_mode);
    g_raw_mode = 0;
    sqx_write_bool(1);
}

/* ---- Input Functions ---- */

static void cmd_read_key(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    DWORD events_read = 0;
    INPUT_RECORD rec;
    if (!ReadConsoleInput(g_hStdin, &rec, 1, &events_read) || events_read == 0) {
        sqx_write_error("failed to read input event");
        return;
    }
    char json_buf[1024];
    switch (rec.EventType) {
        case KEY_EVENT:
            key_event_to_json(&rec.Event.KeyEvent, json_buf, sizeof(json_buf));
            break;
        case MOUSE_EVENT:
            mouse_event_to_json(&rec.Event.MouseEvent, json_buf, sizeof(json_buf));
            break;
        case WINDOW_BUFFER_SIZE_EVENT:
            resize_event_to_json(&rec.Event.WindowBufferSizeEvent, json_buf, sizeof(json_buf));
            break;
        case FOCUS_EVENT:
            snprintf(json_buf, sizeof(json_buf),
                "\"type\":\"focus\",\"focused\":%s",
                rec.Event.FocusEvent.bSetFocus ? "true" : "false");
            break;
        default:
            snprintf(json_buf, sizeof(json_buf), "\"type\":\"unknown\",\"event_type\":%d", rec.EventType);
            break;
    }
    sqx_write_json_object(json_buf);
}

static void cmd_read_keys(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    DWORD events_avail = 0;
    GetNumberOfConsoleInputEvents(g_hStdin, &events_avail);
    if (events_avail == 0) { sqx_write_array(NULL); return; }
    INPUT_RECORD *recs = malloc(events_avail * sizeof(INPUT_RECORD));
    if (!recs) { sqx_write_error("out of memory"); return; }
    DWORD events_read = 0;
    if (!ReadConsoleInput(g_hStdin, recs, events_avail, &events_read)) {
        free(recs);
        sqx_write_error("failed to read input events");
        return;
    }
    size_t total = events_read * 1024 + 4;
    char *json_array = malloc(total);
    if (!json_array) { free(recs); sqx_write_error("out of memory"); return; }
    char *p = json_array;
    *p++ = '[';
    for (DWORD i = 0; i < events_read; i++) {
        if (i > 0) *p++ = ',';
        char event_json[1024];
        switch (recs[i].EventType) {
            case KEY_EVENT:
                key_event_to_json(&recs[i].Event.KeyEvent, event_json, sizeof(event_json));
                break;
            case MOUSE_EVENT:
                mouse_event_to_json(&recs[i].Event.MouseEvent, event_json, sizeof(event_json));
                break;
            case WINDOW_BUFFER_SIZE_EVENT:
                resize_event_to_json(&recs[i].Event.WindowBufferSizeEvent, event_json, sizeof(event_json));
                break;
            case FOCUS_EVENT:
                snprintf(event_json, sizeof(event_json),
                    "\"type\":\"focus\",\"focused\":%s",
                    recs[i].Event.FocusEvent.bSetFocus ? "true" : "false");
                break;
            default:
                snprintf(event_json, sizeof(event_json),
                    "\"type\":\"unknown\",\"event_type\":%d", recs[i].EventType);
                break;
        }
        size_t len = strlen(event_json);
        memcpy(p, event_json, len);
        p += len;
    }
    *p++ = ']';
    *p = '\0';
    sqx_write_structured(1, json_array, NULL);
    free(json_array);
    free(recs);
}

static void cmd_poll_event(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    DWORD events_avail = 0;
    GetNumberOfConsoleInputEvents(g_hStdin, &events_avail);
    sqx_write_bool(events_avail > 0);
}

static void cmd_flush_input(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    FlushConsoleInputBuffer(g_hStdin);
    sqx_write_bool(1);
}

/* ---- Manifest Declaration ---- */
SQX_BEGIN_MANIFEST
    SQX_REGISTER(write, "structured")
    SQX_REGISTER(writeln, "structured")
    SQX_REGISTER(clear, "structured")
    SQX_REGISTER(move, "structured")
    SQX_REGISTER(write_at, "structured")
    SQX_REGISTER(set_fg, "structured")
    SQX_REGISTER(set_bg, "structured")
    SQX_REGISTER(reset_style, "structured")
    SQX_REGISTER(get_size, "structured")
    SQX_REGISTER(cursor_pos, "structured")
    SQX_REGISTER(enable_raw_mode, "structured")
    SQX_REGISTER(disable_raw_mode, "structured")
    SQX_REGISTER(read_key, "structured")
    SQX_REGISTER(read_keys, "structured")
    SQX_REGISTER(poll_event, "structured")
    SQX_REGISTER(flush_input, "structured")
    SQX_REGISTER(enter_alt_screen, "structured")
    SQX_REGISTER(exit_alt_screen, "structured")
    SQX_REGISTER(show_cursor, "structured")
    SQX_REGISTER(scroll, "structured")
    SQX_REGISTER(set_title, "structured")
    SQX_REGISTER(set_size, "structured")
SQX_END_MANIFEST

/* ---- Main Entry Point ---- */
int main(int argc, char **argv) {
    return sqx_main(argc, argv);
}