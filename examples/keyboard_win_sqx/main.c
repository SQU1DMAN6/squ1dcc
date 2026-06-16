/*
 * keyboard_win_sqx — Windows Low-Level Keyboard SQX Module (v1.9)
 *
 * Provides low-level Windows console keyboard input for SQU1DLang.
 * Uses the SQX structured return contract and SQX Runtime Library.
 *
 * NOTE: All functions here are also available in terminal_win_sqx.
 * This module exists as a standalone for scenarios where only
 * keyboard input is needed without the full terminal API.
 *
 * BUILD (on Windows with MSVC/MinGW):
 *   cl /O2 /I"..\..\src\include" keyboard_win.sqx main.c
 *   gcc -O2 -I"../../src/include" -o keyboard_win.sqx main.c
 *
 * USAGE (from SQU1DLang):
 *   var kb = pkg.load_sqx("path/to/keyboard_win.sqx")
 *   kb.enable_raw_input()
 *   var evt = kb.read_key()
 *   kb.disable_raw_input()
 */

#include "sqx_runtime.h"

/* ---- Keyboard State ---- */
static HANDLE g_hStdin = NULL;
static int    g_raw_input = 0;
static DWORD  g_original_mode = 0;
static int    g_initialized = 0;

/* Key name lookup */
typedef struct { WORD vk; const char *name; } KeyName;

static KeyName g_key_names[] = {
    {VK_BACK,      "backspace"}, {VK_TAB,       "tab"},
    {VK_RETURN,    "enter"},     {VK_SHIFT,     "shift"},
    {VK_CONTROL,   "ctrl"},      {VK_MENU,      "alt"},
    {VK_ESCAPE,    "escape"},    {VK_SPACE,     "space"},
    {VK_LEFT,      "left"},      {VK_UP,        "up"},
    {VK_RIGHT,     "right"},     {VK_DOWN,      "down"},
    {VK_PRIOR,     "page_up"},   {VK_NEXT,      "page_down"},
    {VK_HOME,      "home"},      {VK_END,       "end"},
    {VK_INSERT,    "insert"},    {VK_DELETE,    "delete"},
    {VK_SNAPSHOT,  "print_screen"}, {VK_HELP,    "help"},
    {VK_F1,        "f1"},        {VK_F2,        "f2"},
    {VK_F3,        "f3"},        {VK_F4,        "f4"},
    {VK_F5,        "f5"},        {VK_F6,        "f6"},
    {VK_F7,        "f7"},        {VK_F8,        "f8"},
    {VK_F9,        "f9"},        {VK_F10,       "f10"},
    {VK_F11,       "f11"},       {VK_F12,       "f12"},
    {VK_NUMLOCK,   "num_lock"},  {VK_SCROLL,    "scroll_lock"},
    {VK_CAPITAL,   "caps_lock"}, {VK_SLEEP,     "sleep"},
    {VK_LWIN,      "lwin"},      {VK_RWIN,      "rwin"},
    {VK_APPS,      "apps"},
    {0, NULL}
};

static const char* vk_to_name(WORD vk) {
    if (vk >= '0' && vk <= '9') { static char b[2]; b[0]=(char)vk; return b; }
    if (vk >= 'A' && vk <= 'Z') { static char b[2]; b[0]=(char)(vk-'A'+'a'); return b; }
    for (int i = 0; g_key_names[i].name; i++)
        if (g_key_names[i].vk == vk) return g_key_names[i].name;
    if (vk >= VK_OEM_1 && vk <= VK_OEM_102) {
        switch (vk) {
            case VK_OEM_1: return ";"; case VK_OEM_PLUS: return "=";
            case VK_OEM_COMMA: return ","; case VK_OEM_MINUS: return "-";
            case VK_OEM_PERIOD: return "."; case VK_OEM_2: return "/";
            case VK_OEM_3: return "`"; case VK_OEM_4: return "[";
            case VK_OEM_5: return "\\"; case VK_OEM_6: return "]";
            case VK_OEM_7: return "'"; default: break;
        }
    }
    static char buf[16]; snprintf(buf,sizeof(buf),"vk_0x%02x",vk); return buf;
}

static void ensure_handles(void) {
    if (g_initialized) return;
    g_hStdin = GetStdHandle(STD_INPUT_HANDLE);
    g_initialized = 1;
}

static void append_modifiers(char *buf, size_t buf_size, DWORD ctrl_key_state) {
    char *p = buf; *p++ = '[';
    int first = 1;
    if (ctrl_key_state & SHIFT_PRESSED) {
        p += snprintf(p, buf_size-(p-buf), "%s\"shift\"", first?"":","); first=0; }
    if (ctrl_key_state & (LEFT_CTRL_PRESSED|RIGHT_CTRL_PRESSED)) {
        p += snprintf(p, buf_size-(p-buf), "%s\"ctrl\"", first?"":","); first=0; }
    if (ctrl_key_state & (LEFT_ALT_PRESSED|RIGHT_ALT_PRESSED)) {
        p += snprintf(p, buf_size-(p-buf), "%s\"alt\"", first?"":","); first=0; }
    *p++ = ']'; *p = '\0';
}

static void key_event_to_json(const KEY_EVENT_RECORD *ker, char *buf, size_t buf_size) {
    const char *key_name = vk_to_name(ker->wVirtualKeyCode);
    char uchar[5] = {0};
    if (ker->uChar.AsciiChar) uchar[0] = ker->uChar.AsciiChar;
    char mod_json[64];
    append_modifiers(mod_json,sizeof(mod_json),ker->dwControlKeyState);
    snprintf(buf,buf_size,
        "\"type\":\"key\",\"key\":\"%s\",\"char\":\"%s\",\"pressed\":%s,"
        "\"repeat_count\":%d,\"vk_code\":%u,\"modifiers\":%s,"
        "\"ctrl\":%s,\"alt\":%s,\"shift\":%s",
        key_name, uchar, ker->bKeyDown?"true":"false",
        ker->wRepeatCount, ker->wVirtualKeyCode, mod_json,
        (ker->dwControlKeyState&(LEFT_CTRL_PRESSED|RIGHT_CTRL_PRESSED))?"true":"false",
        (ker->dwControlKeyState&(LEFT_ALT_PRESSED|RIGHT_ALT_PRESSED))?"true":"false",
        (ker->dwControlKeyState&SHIFT_PRESSED)?"true":"false");
}

/* ---- SQX Function Handlers ---- */

static void cmd_enable_raw_input(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    if (g_raw_input) { sqx_write_bool(1); return; }
    if (!GetConsoleMode(g_hStdin, &g_original_mode)) {
        sqx_write_error("failed to get console mode"); return;
    }
    DWORD new_mode = ENABLE_EXTENDED_FLAGS;
    if (!SetConsoleMode(g_hStdin, new_mode)) {
        sqx_write_error("failed to set raw input mode"); return;
    }
    new_mode = ENABLE_VIRTUAL_TERMINAL_INPUT | ENABLE_MOUSE_INPUT | ENABLE_WINDOW_INPUT;
    SetConsoleMode(g_hStdin, new_mode);
    g_raw_input = 1;
    sqx_write_bool(1);
}

static void cmd_disable_raw_input(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    if (!g_raw_input) { sqx_write_bool(1); return; }
    SetConsoleMode(g_hStdin, g_original_mode);
    g_raw_input = 0;
    sqx_write_bool(1);
}

static void cmd_read_key(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    if (!g_raw_input) { sqx_write_error("raw input not enabled; call enable_raw_input() first"); return; }
    DWORD events_read = 0;
    INPUT_RECORD rec;
    if (!ReadConsoleInput(g_hStdin, &rec, 1, &events_read) || events_read == 0) {
        sqx_write_error("failed to read input event"); return;
    }
    char json_buf[1024];
    switch (rec.EventType) {
        case KEY_EVENT:
            key_event_to_json(&rec.Event.KeyEvent, json_buf, sizeof(json_buf)); break;
        case MOUSE_EVENT: {
            const MOUSE_EVENT_RECORD *mer = &rec.Event.MouseEvent;
            const char *button = "none";
            if (mer->dwButtonState & FROM_LEFT_1ST_BUTTON_PRESSED) button = "left";
            else if (mer->dwButtonState & RIGHTMOST_BUTTON_PRESSED) button = "right";
            const char *et = "mouse_move";
            if (mer->dwEventFlags & DOUBLE_CLICK) et = "mouse_double_click";
            else if (mer->dwEventFlags & MOUSE_WHEELED) et = "mouse_wheel";
            char mod[64]; append_modifiers(mod,sizeof(mod),mer->dwControlKeyState);
            snprintf(json_buf,sizeof(json_buf),
                "\"type\":\"mouse\",\"event\":\"%s\",\"x\":%d,\"y\":%d,\"button\":\"%s\",\"modifiers\":%s",
                et, mer->dwMousePosition.X, mer->dwMousePosition.Y, button, mod);
            break;
        }
        case WINDOW_BUFFER_SIZE_EVENT:
            snprintf(json_buf,sizeof(json_buf),
                "\"type\":\"resize\",\"width\":%d,\"height\":%d",
                rec.Event.WindowBufferSizeEvent.dwSize.X,
                rec.Event.WindowBufferSizeEvent.dwSize.Y);
            break;
        default:
            snprintf(json_buf,sizeof(json_buf),"\"type\":\"unknown\"");
            break;
    }
    sqx_write_json_object(json_buf);
}

static void cmd_read_keys(int argc, char **argv) {
    (void)argc; (void)argv;
    ensure_handles();
    if (!g_raw_input) { sqx_write_error("raw input not enabled"); return; }
    DWORD events_avail = 0;
    GetNumberOfConsoleInputEvents(g_hStdin, &events_avail);
    if (events_avail == 0) { sqx_write_array(NULL); return; }
    INPUT_RECORD *recs = malloc(events_avail * sizeof(INPUT_RECORD));
    if (!recs) { sqx_write_error("out of memory"); return; }
    DWORD events_read = 0;
    if (!ReadConsoleInput(g_hStdin, recs, events_avail, &events_read)) {
        free(recs); sqx_write_error("failed to read"); return;
    }
    size_t total = events_read * 1024 + 4;
    char *arr = malloc(total);
    if (!arr) { free(recs); sqx_write_error("out of memory"); return; }
    char *p = arr; *p++ = '[';
    for (DWORD i = 0; i < events_read; i++) {
        if (i > 0) *p++ = ',';
        char ej[1024]; key_event_to_json(&recs[i].Event.KeyEvent, ej, sizeof(ej));
        size_t l = strlen(ej); memcpy(p, ej, l); p += l;
    }
    *p++ = ']'; *p = '\0';
    sqx_write_structured(1, arr, NULL);
    free(arr); free(recs);
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

static void cmd_get_key_state(int argc, char **argv) {
    if (argc < 1) { sqx_write_error("get_key_state requires 1 argument"); return; }
    WORD vk = (WORD)atoi(argv[0]);
    SHORT state = GetAsyncKeyState(vk);
    int pressed = (state & 0x8000) != 0;
    const char *name = vk_to_name(vk);
    char buf[128];
    snprintf(buf, sizeof(buf), "\"key\":\"%s\",\"vk_code\":%u,\"pressed\":%s",
        name, vk, pressed ? "true" : "false");
    sqx_write_json_object(buf);
}

static void cmd_get_pressed_keys(int argc, char **argv) {
    (void)argc; (void)argv;
    char buf[4096]; char *p = buf; *p++ = '[';
    int first = 1;
    for (WORD vk = 1; vk < 256; vk++) {
        if (GetAsyncKeyState(vk) & 0x8000) {
            if (!first) *p++ = ','; first = 0;
            const char *name = vk_to_name(vk);
            p += snprintf(p, buf+sizeof(buf)-p, "{\"key\":\"%s\",\"vk_code\":%u}", name, vk);
        }
    }
    *p++ = ']'; *p = '\0';
    sqx_write_structured(1, buf, NULL);
}

/* ---- Manifest Declaration ---- */
SQX_BEGIN_MANIFEST
    SQX_REGISTER(enable_raw_input, "structured")
    SQX_REGISTER(disable_raw_input, "structured")
    SQX_REGISTER(read_key, "structured")
    SQX_REGISTER(read_keys, "structured")
    SQX_REGISTER(poll_event, "structured")
    SQX_REGISTER(flush_input, "structured")
    SQX_REGISTER(get_key_state, "structured")
    SQX_REGISTER(get_pressed_keys, "structured")
SQX_END_MANIFEST

/* ---- Main Entry Point ---- */
int main(int argc, char **argv) {
    return sqx_main(argc, argv);
}