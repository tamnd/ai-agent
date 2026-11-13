# SPDX-License-Identifier: MIT
# Part of https://ai.tamnd.com/binary-search/. The authors are listed in prov/2B7QDX.jsonl.

import random

from binary_search import binary_search


def test_examples() -> None:
    a = [2, 5, 8, 12, 16, 23, 38, 56, 72, 91]
    assert binary_search(a, 23) == 5
    assert binary_search(a, 2) == 0
    assert binary_search(a, 91) == 9
    assert binary_search(a, 30) == -1
    assert binary_search(a, 1) == -1
    assert binary_search(a, 100) == -1


def test_empty_and_single() -> None:
    assert binary_search([], 1) == -1
    assert binary_search([7], 7) == 0
    assert binary_search([7], 3) == -1


def test_duplicates() -> None:
    a = [1, 3, 3, 3, 5]
    assert a[binary_search(a, 3)] == 3


def test_strings() -> None:
    assert binary_search(["ant", "bee", "cat"], "bee") == 1


def test_random_against_linear_scan() -> None:
    rng = random.Random(2150)
    for _ in range(2000):
        a = sorted(rng.randint(-20, 20) for _ in range(rng.randint(0, 30)))
        x = rng.randint(-25, 25)
        i = binary_search(a, x)
        if x in a:
            assert 0 <= i < len(a) and a[i] == x
        else:
            assert i == -1
