/*
 * SQX Runtime Library for C — Structured SQX Module Framework (v1.9)
 *
 * Provides:
 *   - Structured JSON return contract ({ok, value, error})
 *   - Legacy CLI dispatch (__sqx_manifest__, __sqx_call__)
 *   - Persistent session mode (JSON-over-stdin/stdout)
 *   - JSON string escaping utilities
 *
 * USAGE:
 *   1. Include this header in your C SQX module.
 *   2. Register your functions using SQX_REGISTER().
 *   3. Call sqx_main(argc, argv) as your main() dispatcher.
 *
 * EXAMPLE (main.c):
 *   #include "sqx_runtime.h"
 *   static void cmd_ping(int argc, char **argv) {
 *       (void)argc; (void)argv;
 *       sqx_write_string("pong");
 *   }
 *   SQX_BEGIN_MANIFEST
 *       SQX_REGISTER(ping, "string")
 *   SQX_END_MANIFEST
 *   int main(int argc, char **argv) {
 *       return sqx_main(argc, argv);
 *   }
 */

#ifndef SQX_RUNTIME_H
#define SQX_RUNTIME_H

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <stdarg.h>

/* ---- JSON Escaping ---- */

/* sqx_escape_json: Escape a string for safe inclusion in JSON.
 * Returns a malloc'd escaped string (caller must free).
 * Returns "" (static) on NULL input or allocation failure.
 */
static const char* sqx_escape_json(const char *raw) {
    if (raw == NULL) return "";

    size_t raw_len = strlen(raw);
    /* Worst case: every char needs escaping (double, backslash, etc.) -> 2x + 3 */
    char *escaped = malloc(raw_len * 2 + 3);
    if (!escaped) return "";

    char *p = escaped;
    for (const char *s = raw; *s; s++) {
        unsigned char c = (unsigned char)*s;
        switch (c) {
            case '"':  *p++ = '\\'; *p++ = '"';  break;
            case '\\': *p++ = '\\'; *p++ = '\\'; break;
            case '\b': *p++ = '\\'; *p++ = 'b';  break;
            case '\f': *p++ = '\\'; *p++ = 'f';  break;
            case '\n': *p++ = '\\'; *p++ = 'n';  break;
            case '\r': *p++ = '\\'; *p++ = 'r';  break;
            case '\t': *p++ = '\\'; *p++ = 't';  break;
            default:
                if (c < 0x20) {
                    /* Control characters: \u00XX */
                    p += sprintf(p, "\\u%04x", c);
                } else {
                    *p++ = c;
                }
                break;
        }
    }
    *p = '\0';
    return escaped;
}

/* ---- Structured JSON Output ---- */

/* sqx_write_structured: Write a complete structured JSON result to stdout.
 *   ok:        1 for success, 0 for error
 *   value_json: JSON-encoded value string (or NULL for null)
 *   error_msg:  Error message string (or NULL for no error)
 *
 * Output format: {"ok":true,"value":... ,"error":null}\n
 */
static void sqx_write_structured(int ok, const char *value_json, const char *error_msg) {
    printf("{\"ok\":%s,\"value\":%s,\"error\":%s}\n",
           ok ? "true" : "false",
           value_json ? value_json : "null",
           error_msg ? error_msg : "null");
    fflush(stdout);
}

/* sqx_write_error: Write a structured error result.
 *   msg: Error message (will be JSON-escaped automatically)
 */
static void sqx_write_error(const char *msg) {
    const char *escaped = sqx_escape_json(msg);
    char buf[1024];
    snprintf(buf, sizeof(buf), "\"%s\"", escaped);
    if (escaped != msg && escaped != NULL && escaped != (void*)"" && escaped[0] != '\0') {
        /* Free only if sqx_escape_json allocated new memory */
        /* We can't portably determine this, so just use the buffer */
    }
    sqx_write_structured(0, NULL, buf);
}

/* sqx_write_string: Write a structured success result with a string value.
 *   str: String value (will be JSON-escaped automatically)
 */
static void sqx_write_string(const char *str) {
    if (str == NULL) {
        sqx_write_structured(1, "\"\"", NULL);
        return;
    }
    const char *escaped = sqx_escape_json(str);
    printf("{\"ok\":true,\"value\":\"%s\",\"error\":null}\n", escaped);
    fflush(stdout);
    free((void*)escaped);
}

/* sqx_write_int: Write a structured success result with an integer value. */
static void sqx_write_int(long long val) {
    char buf[32];
    snprintf(buf, sizeof(buf), "%lld", val);
    sqx_write_structured(1, buf, NULL);
}

/* sqx_write_bool: Write a structured success result with a boolean value. */
static void sqx_write_bool(int val) {
    sqx_write_structured(1, val ? "true" : "false", NULL);
}

/* sqx_write_null: Write a structured success result with a null value. */
static void sqx_write_null(void) {
    sqx_write_structured(1, "null", NULL);
}

/* sqx_write_json: Write a structured success result with a raw JSON value.
 *   json_body: Pre-formatted JSON value string (NOT including surrounding braces).
 *              If the string starts with '{', it is treated as a complete JSON object body.
 *              Otherwise it is treated as a JSON primitive value.
 */
static void sqx_write_json(const char *json_body) {
    if (json_body == NULL || json_body[0] == '\0') {
        sqx_write_structured(1, "null", NULL);
        return;
    }
    /* If it's already an object/array/primitive, use it directly */
    sqx_write_structured(1, json_body, NULL);
}

/* sqx_write_json_object: Write a structured success result with a JSON object.
 *   json_object_body: Comma-separated key:value pairs (without outer braces).
 *                     Example: "\"width\":80,\"height\":25"
 */
static void sqx_write_json_object(const char *json_object_body) {
    size_t len = strlen(json_object_body);
    char *buf = malloc(len + 4);
    if (!buf) { sqx_write_error("out of memory"); return; }
    buf[0] = '{';
    memcpy(buf + 1, json_object_body, len);
    buf[len + 1] = '}';
    buf[len + 2] = '\0';
    sqx_write_structured(1, buf, NULL);
    free(buf);
}

/* sqx_write_array: Write a structured success result with a JSON array.
 *   json_array_body: Comma-separated values (without outer brackets).
 *                    If NULL or empty, writes an empty array.
 */
static void sqx_write_array(const char *json_array_body) {
    if (json_array_body == NULL || json_array_body[0] == '\0') {
        sqx_write_structured(1, "[]", NULL);
        return;
    }
    size_t len = strlen(json_array_body);
    char *buf = malloc(len + 4);
    if (!buf) { sqx_write_error("out of memory"); return; }
    buf[0] = '[';
    memcpy(buf + 1, json_array_body, len);
    buf[len + 1] = ']';
    buf[len + 2] = '\0';
    sqx_write_structured(1, buf, NULL);
    free(buf);
}

/* ---- Manifest Registration ---- */

/* Maximum number of registered SQX functions */
#define SQX_MAX_FUNCTIONS 64

/* Function handler type: receives arguments and must call sqx_write_*() */
typedef void (*sqx_handler_fn)(int argc, char **argv);

/* Registered function entry */
typedef struct {
    const char     *name;
    const char     *return_mode;
    sqx_handler_fn  handler;
} sqx_function_entry;

/* Global registry */
static sqx_function_entry sqx_functions[SQX_MAX_FUNCTIONS];
static int               sqx_function_count = 0;

/* SQX_REGISTER: Register an SQX function handler.
 *   fn_name: Function name as exposed to SQU1DLang (e.g., "write")
 *   ret_mode: Return type mode ("structured", "string", "int", "bool", "json", "auto", etc.)
 *   handler: Function pointer to the handler implementation.
 *
 * Usage (inside SQX_BEGIN_MANIFEST / SQX_END_MANIFEST):
 *   SQX_REGISTER(ping, "string")
 */
#define SQX_REGISTER(fn_name, ret_mode) \
    do { \
        if (sqx_function_count < SQX_MAX_FUNCTIONS) { \
            sqx_functions[sqx_function_count].name        = #fn_name; \
            sqx_functions[sqx_function_count].return_mode = (ret_mode); \
            sqx_functions[sqx_function_count].handler     = cmd_##fn_name; \
            sqx_function_count++; \
        } \
    } while (0)

/* SQX_BEGIN_MANIFEST / SQX_END_MANIFEST: Declare the function manifest block.
 * Place SQX_REGISTER calls between them.
 *
 * Example:
 *   SQX_BEGIN_MANIFEST
 *       SQX_REGISTER(ping, "string")
 *       SQX_REGISTER(write, "structured")
 *   SQX_END_MANIFEST
 */
#define SQX_BEGIN_MANIFEST \
    static void sqx_build_manifest(void) { \
        (void)sqx_build_manifest;

#define SQX_END_MANIFEST \
    }

/* ---- Manifest Output (Legacy Mode) ---- */

/* sqx_print_manifest: Print JSON manifest to stdout.
 * Called when module is invoked with __sqx_manifest__.
 */
static void sqx_print_manifest(void) {
    printf("{\"version\":1,\"functions\":{");
    for (int i = 0; i < sqx_function_count; i++) {
        if (i > 0) printf(",");
        printf("\"%s\":{\"return\":\"%s\"}",
               sqx_functions[i].name,
               sqx_functions[i].return_mode);
    }
    printf("}}\n");
    fflush(stdout);
}

/* ---- Function Dispatch (Legacy Mode) ---- */

/* sqx_call: Dispatch a function call in legacy mode.
 *   fn: Function name to call
 *   argc: Number of arguments
 *   argv: Argument array
 * Returns exit code (0 on success, 2 on unknown function).
 */
static int sqx_call(const char *fn, int argc, char **argv) {
    for (int i = 0; i < sqx_function_count; i++) {
        if (strcmp(sqx_functions[i].name, fn) == 0) {
            sqx_functions[i].handler(argc, argv);
            return 0;
        }
    }
    fprintf(stderr, "unknown SQX function: %s\n", fn);
    return 2;
}

/* ---- Session Mode ---- */

/* sqx_serve_session: Enter persistent session mode.
 * Reads JSON requests from stdin, dispatches to registered handlers,
 * and writes JSON responses to stdout.
 *
 * Request format: {"cmd":"call","fn":"func_name","args":["arg1","arg2"]}
 * Response format: {"ok":true,"value":... ,"error":null}
 *
 * Also supports:
 *   {"cmd":"ping"}           -> {"ok":true,"value":"pong","error":null}
 *   {"cmd":"shutdown"}       -> exits session loop
 *
 * Returns exit code (0 on clean shutdown).
 */
static int sqx_serve_session(void) {
    char line[65536];
    while (fgets(line, sizeof(line), stdin)) {
        /* Trim trailing newline/whitespace */
        size_t len = strlen(line);
        while (len > 0 && (line[len-1] == '\n' || line[len-1] == '\r' || line[len-1] == ' ')) {
            line[--len] = '\0';
        }

        /* Parse command */
        const char *cmd_start = NULL;
        const char *fn_start = NULL;
        const char *args_start = NULL;

        /* Simple JSON parsing — look for "cmd":, "fn":, "args": */
        const char *p = line;
        while (*p) {
            /* Skip whitespace */
            while (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\r') p++;

            /* Look for "cmd" */
            if (strncmp(p, "\"cmd\"", 5) == 0) {
                p += 5;
                while (*p == ' ' || *p == ':' || *p == '\t') p++;
                if (*p == '"') {
                    p++;
                    cmd_start = p;
                    while (*p && *p != '"') p++;
                }
                continue;
            }

            /* Look for "fn" */
            if (strncmp(p, "\"fn\"", 4) == 0) {
                p += 4;
                while (*p == ' ' || *p == ':' || *p == '\t') p++;
                if (*p == '"') {
                    p++;
                    fn_start = p;
                    while (*p && *p != '"') p++;
                }
                continue;
            }

            /* Look for "args" */
            if (strncmp(p, "\"args\"", 6) == 0) {
                p += 6;
                while (*p == ' ' || *p == ':' || *p == '\t') p++;
                args_start = p;
                /* Skip past the JSON array (find matching ']') */
                int depth = 0;
                while (*p) {
                    if (*p == '[') depth++;
                    if (*p == ']') {
                        depth--;
                        if (depth == 0) { p++; break; }
                    }
                    p++;
                }
                continue;
            }

            p++;
        }

        if (!cmd_start) {
            sqx_write_error("malformed request: missing cmd field");
            continue;
        }

        /* Extract command string (null-terminate in-place) */
        size_t cmd_len = strcspn(cmd_start, "\"");
        char cmd_buf[64];
        size_t copy_len = cmd_len < sizeof(cmd_buf)-1 ? cmd_len : sizeof(cmd_buf)-1;
        strncpy(cmd_buf, cmd_start, copy_len);
        cmd_buf[copy_len] = '\0';

        if (strcmp(cmd_buf, "call") == 0) {
            if (!fn_start) {
                sqx_write_error("malformed request: missing fn field for call command");
                continue;
            }

            /* Extract function name */
            size_t fn_len = strcspn(fn_start, "\"");
            char fn_buf[128];
            copy_len = fn_len < sizeof(fn_buf)-1 ? fn_len : sizeof(fn_buf)-1;
            strncpy(fn_buf, fn_start, copy_len);
            fn_buf[copy_len] = '\0';

            /* Parse arguments array if present */
            int arg_count = 0;
            char *arg_values[256];

            if (args_start) {
                /* Simple array of strings parser */
                const char *ap = args_start;
                while (*ap && *ap != ']') {
                    /* Skip whitespace and brackets/commas */
                    while (*ap == ' ' || *ap == '\t' || *ap == '[' || *ap == ',') ap++;
                    if (*ap == ']' || *ap == '\0') break;

                    if (*ap == '"') {
                        ap++; /* Skip opening quote */
                        const char *str_start = ap;
                        while (*ap && *ap != '"') {
                            if (*ap == '\\') ap++; /* Skip escaped char */
                            ap++;
                        }
                        size_t str_len = ap - str_start;
                        if (arg_count < 256) {
                            char *arg_copy = malloc(str_len + 1);
                            if (arg_copy) {
                                strncpy(arg_copy, str_start, str_len);
                                arg_copy[str_len] = '\0';
                                arg_values[arg_count++] = arg_copy;
                            }
                        }
                        if (*ap) ap++; /* Skip closing quote */
                    } else {
                        /* Number or boolean */
                        const char *val_start = ap;
                        while (*ap && *ap != ',' && *ap != ']' && *ap != ' ') ap++;
                        size_t val_len = ap - val_start;
                        if (arg_count < 256 && val_len > 0) {
                            char *arg_copy = malloc(val_len + 1);
                            if (arg_copy) {
                                strncpy(arg_copy, val_start, val_len);
                                arg_copy[val_len] = '\0';
                                arg_values[arg_count++] = arg_copy;
                            }
                        }
                    }
                }
            }

            /* Find and call the function */
            int found = 0;
            for (int i = 0; i < sqx_function_count; i++) {
                if (strcmp(sqx_functions[i].name, fn_buf) == 0) {
                    sqx_functions[i].handler(arg_count, arg_values);
                    found = 1;
                    break;
                }
            }

            /* Free argument strings */
            for (int i = 0; i < arg_count; i++) {
                free(arg_values[i]);
            }

            if (!found) {
                char err_buf[256];
                snprintf(err_buf, sizeof(err_buf), "unknown SQX function: %s", fn_buf);
                sqx_write_error(err_buf);
            }

        } else if (strcmp(cmd_buf, "ping") == 0) {
            sqx_write_string("pong");

        } else if (strcmp(cmd_buf, "shutdown") == 0) {
            return 0;

        } else {
            char err_buf[256];
            snprintf(err_buf, sizeof(err_buf), "unknown command: %s", cmd_buf);
            sqx_write_error(err_buf);
        }
    }

    return 0;
}

/* ---- Main Dispatcher ---- */

/* sqx_main: Main entry point for SQX modules.
 * Handles legacy mode (__sqx_manifest__, __sqx_call__) and session mode (--session).
 *
 * Call this from your main():
 *   int main(int argc, char **argv) {
 *       return sqx_main(argc, argv);
 *   }
 */
static int sqx_main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <__sqx_manifest__|__sqx_call__|--session>\n",
                argv[0] ? argv[0] : "sqx_module");
        return 2;
    }

    /* Build manifest from SQX_REGISTER calls */
    sqx_build_manifest();

    if (strcmp(argv[1], "--session") == 0) {
        return sqx_serve_session();
    }

    if (strcmp(argv[1], "__sqx_manifest__") == 0) {
        sqx_print_manifest();
        return 0;
    }

    if (strcmp(argv[1], "__sqx_call__") == 0) {
        if (argc < 3) {
            fprintf(stderr, "missing SQX function name\n");
            return 2;
        }
        return sqx_call(argv[2], argc - 3, argv + 3);
    }

    fprintf(stderr, "unknown SQX command: %s\n", argv[1]);
    return 2;
}

#endif /* SQX_RUNTIME_H */