// SPDX-License-Identifier: MIT
// A small JSON reader and writer for the C adapters of https://algorithm.tamnd.com (spec 09.2).
#ifndef ALGO_JSON_H
#define ALGO_JSON_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

typedef enum { AJ_NULL, AJ_BOOL, AJ_NUM, AJ_STR, AJ_ARR, AJ_OBJ } aj_kind;

typedef struct aj {
    aj_kind kind;
    bool b;
    char *s;           // the text of a number or a string
    size_t len;        // the length of s, or the number of items
    struct aj *items;  // the items of a list, or the values of an object
    char **keys;       // the keys of an object
} aj;

// aj_parse reads one JSON value. It returns NULL when the text is not JSON.
aj *aj_parse(const char *text);
void aj_free(aj *v);
// aj_get returns the value of key in an object, or NULL.
const aj *aj_get(const aj *obj, const char *key);
int64_t aj_int(const aj *v);
// aj_int_list returns a new array of the integers of a list. The caller frees it.
int64_t *aj_int_list(const aj *v, size_t *n);
void aj_put_str(const char *s, size_t n);
void aj_put_int(long long x);
void aj_put_int_list(const int64_t *a, size_t n);

#endif
