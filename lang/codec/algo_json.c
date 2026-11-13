// SPDX-License-Identifier: MIT
// A small JSON reader and writer for the C adapters of https://algorithm.tamnd.com (spec 09.2).
#include "algo_json.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    const char *p;
    bool bad;
} parser;

static void ws(parser *ps) {
    while (*ps->p == ' ' || *ps->p == '\t' || *ps->p == '\n' || *ps->p == '\r') ps->p++;
}

static void *grow(void *a, size_t *cap, size_t n, size_t size) {
    if (n < *cap) return a;
    *cap = *cap ? *cap * 2 : 8;
    void *b = realloc(a, *cap * size);
    if (!b) abort();
    return b;
}

static void push(char **out, size_t *cap, size_t *n, char c) {
    *out = grow(*out, cap, *n + 1, 1);
    (*out)[(*n)++] = c;
}

static void put_utf8(char **out, size_t *cap, size_t *n, unsigned cp) {
    if (cp < 0x80) {
        push(out, cap, n, (char)cp);
    } else if (cp < 0x800) {
        push(out, cap, n, (char)(0xC0 | (cp >> 6)));
        push(out, cap, n, (char)(0x80 | (cp & 0x3F)));
    } else if (cp < 0x10000) {
        push(out, cap, n, (char)(0xE0 | (cp >> 12)));
        push(out, cap, n, (char)(0x80 | ((cp >> 6) & 0x3F)));
        push(out, cap, n, (char)(0x80 | (cp & 0x3F)));
    } else {
        push(out, cap, n, (char)(0xF0 | (cp >> 18)));
        push(out, cap, n, (char)(0x80 | ((cp >> 12) & 0x3F)));
        push(out, cap, n, (char)(0x80 | ((cp >> 6) & 0x3F)));
        push(out, cap, n, (char)(0x80 | (cp & 0x3F)));
    }
}

static unsigned hex4(parser *ps) {
    unsigned v = 0;
    for (int i = 0; i < 4; i++) {
        char c = *ps->p;
        if (c >= '0' && c <= '9') v = v * 16 + (unsigned)(c - '0');
        else if (c >= 'a' && c <= 'f') v = v * 16 + (unsigned)(c - 'a' + 10);
        else if (c >= 'A' && c <= 'F') v = v * 16 + (unsigned)(c - 'A' + 10);
        else { ps->bad = true; return 0; }
        ps->p++;
    }
    return v;
}

static char *string(parser *ps, size_t *len) {
    char *out = NULL;
    size_t cap = 0, n = 0;
    ps->p++;
    while (*ps->p && *ps->p != '"') {
        char c = *ps->p++;
        if (c != '\\') {
            push(&out, &cap, &n, c);
            continue;
        }
        char e = *ps->p++;
        switch (e) {
        case 'n': push(&out, &cap, &n, '\n'); break;
        case 't': push(&out, &cap, &n, '\t'); break;
        case 'r': push(&out, &cap, &n, '\r'); break;
        case 'b': push(&out, &cap, &n, '\b'); break;
        case 'f': push(&out, &cap, &n, '\f'); break;
        case 'u': {
            unsigned cp = hex4(ps);
            if (cp >= 0xD800 && cp < 0xDC00 && ps->p[0] == '\\' && ps->p[1] == 'u') {
                ps->p += 2;
                unsigned lo = hex4(ps);
                cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
            }
            put_utf8(&out, &cap, &n, cp);
            break;
        }
        case '\0': ps->bad = true; ps->p--; break;
        default: push(&out, &cap, &n, e);
        }
    }
    if (*ps->p != '"') ps->bad = true;
    else ps->p++;
    push(&out, &cap, &n, '\0');
    *len = n - 1;
    return out;
}

static void value(parser *ps, aj *v);

static void list(parser *ps, aj *v, bool obj) {
    size_t cap = 0, kcap = 0;
    v->kind = obj ? AJ_OBJ : AJ_ARR;
    ps->p++;
    ws(ps);
    if (*ps->p == (obj ? '}' : ']')) {
        ps->p++;
        return;
    }
    for (;;) {
        ws(ps);
        if (obj) {
            if (*ps->p != '"') { ps->bad = true; return; }
            size_t klen;
            char *k = string(ps, &klen);
            v->keys = grow(v->keys, &kcap, v->len + 1, sizeof(char *));
            v->keys[v->len] = k;
            ws(ps);
            if (*ps->p != ':') { free(k); ps->bad = true; return; }
            ps->p++;
        }
        v->items = grow(v->items, &cap, v->len + 1, sizeof(aj));
        memset(&v->items[v->len], 0, sizeof(aj));
        value(ps, &v->items[v->len]);
        v->len++;
        if (ps->bad) return;
        ws(ps);
        if (*ps->p == ',') { ps->p++; continue; }
        if (*ps->p == (obj ? '}' : ']')) { ps->p++; return; }
        ps->bad = true;
        return;
    }
}

static void value(parser *ps, aj *v) {
    ws(ps);
    const char *p = ps->p;
    if (*p == '{') list(ps, v, true);
    else if (*p == '[') list(ps, v, false);
    else if (*p == '"') { v->kind = AJ_STR; v->s = string(ps, &v->len); }
    else if (strncmp(p, "null", 4) == 0) { v->kind = AJ_NULL; ps->p += 4; }
    else if (strncmp(p, "true", 4) == 0) { v->kind = AJ_BOOL; v->b = true; ps->p += 4; }
    else if (strncmp(p, "false", 5) == 0) { v->kind = AJ_BOOL; ps->p += 5; }
    else {
        const char *q = p;
        while (*q == '-' || *q == '+' || *q == '.' || *q == 'e' || *q == 'E' || (*q >= '0' && *q <= '9')) q++;
        if (q == p) { ps->bad = true; return; }
        v->kind = AJ_NUM;
        v->len = (size_t)(q - p);
        v->s = malloc(v->len + 1);
        if (!v->s) abort();
        memcpy(v->s, p, v->len);
        v->s[v->len] = '\0';
        ps->p = q;
    }
}

static void clear(aj *v) {
    free(v->s);
    if (v->kind == AJ_ARR || v->kind == AJ_OBJ) {
        for (size_t i = 0; i < v->len; i++) {
            clear(&v->items[i]);
            if (v->keys) free(v->keys[i]);
        }
        free(v->items);
        free(v->keys);
    }
}

aj *aj_parse(const char *text) {
    parser ps = {text, false};
    aj *v = calloc(1, sizeof(aj));
    if (!v) abort();
    value(&ps, v);
    ws(&ps);
    if (ps.bad || *ps.p != '\0') {
        aj_free(v);
        return NULL;
    }
    return v;
}

void aj_free(aj *v) {
    if (!v) return;
    clear(v);
    free(v);
}

const aj *aj_get(const aj *obj, const char *key) {
    if (!obj || obj->kind != AJ_OBJ) return NULL;
    for (size_t i = 0; i < obj->len; i++)
        if (strcmp(obj->keys[i], key) == 0) return &obj->items[i];
    return NULL;
}

int64_t aj_int(const aj *v) {
    if (!v || (v->kind != AJ_NUM && v->kind != AJ_STR)) return 0;
    return (int64_t)strtoll(v->s, NULL, 10);
}

int64_t *aj_int_list(const aj *v, size_t *n) {
    *n = (v && v->kind == AJ_ARR) ? v->len : 0;
    int64_t *a = calloc(*n ? *n : 1, sizeof(int64_t));
    if (!a) abort();
    for (size_t i = 0; i < *n; i++) a[i] = aj_int(&v->items[i]);
    return a;
}

void aj_put_str(const char *s, size_t n) {
    putchar('"');
    for (size_t i = 0; i < n; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '"' || c == '\\') { putchar('\\'); putchar(c); }
        else if (c == '\n') fputs("\\n", stdout);
        else if (c == '\t') fputs("\\t", stdout);
        else if (c == '\r') fputs("\\r", stdout);
        else if (c < 0x20) printf("\\u%04x", c);
        else putchar(c);
    }
    putchar('"');
}

void aj_put_int(long long x) { printf("%lld", x); }

void aj_put_int_list(const int64_t *a, size_t n) {
    putchar('[');
    for (size_t i = 0; i < n; i++) {
        if (i) putchar(',');
        printf("%lld", (long long)a[i]);
    }
    putchar(']');
}
