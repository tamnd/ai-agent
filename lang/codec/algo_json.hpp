// SPDX-License-Identifier: MIT
// A small JSON reader and writer for the C++ adapters of https://algorithm.tamnd.com (spec 09.2).
#pragma once

#include <concepts>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <optional>
#include <span>
#include <stdexcept>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace algo_json {

struct V {
    enum Kind { Null, Bool, Num, Str, Arr, Obj } kind = Null;
    bool b = false;
    std::string s;  // the text of a number or a string
    std::vector<V> items;
    std::vector<std::string> keys;

    const V& operator[](std::string_view key) const {
        for (std::size_t i = 0; i < keys.size(); i++)
            if (keys[i] == key) return items[i];
        throw std::runtime_error("no key " + std::string(key));
    }
    bool has(std::string_view key) const {
        for (const auto& k : keys)
            if (k == key) return true;
        return false;
    }
};

class Parser {
public:
    explicit Parser(std::string_view s) : s_(s) {}

    V parse() {
        V v = value();
        ws();
        if (i_ != s_.size()) fail();
        return v;
    }

private:
    std::string_view s_;
    std::size_t i_ = 0;

    [[noreturn]] void fail() { throw std::runtime_error("the request is not JSON"); }
    char peek() const { return i_ < s_.size() ? s_[i_] : '\0'; }
    void ws() {
        while (i_ < s_.size() && (s_[i_] == ' ' || s_[i_] == '\t' || s_[i_] == '\n' || s_[i_] == '\r')) i_++;
    }
    unsigned hex4() {
        if (i_ + 4 > s_.size()) fail();
        unsigned v = std::stoul(std::string(s_.substr(i_, 4)), nullptr, 16);
        i_ += 4;
        return v;
    }
    static void utf8(std::string& o, unsigned cp) {
        if (cp < 0x80) {
            o += static_cast<char>(cp);
        } else if (cp < 0x800) {
            o += static_cast<char>(0xC0 | (cp >> 6));
            o += static_cast<char>(0x80 | (cp & 0x3F));
        } else if (cp < 0x10000) {
            o += static_cast<char>(0xE0 | (cp >> 12));
            o += static_cast<char>(0x80 | ((cp >> 6) & 0x3F));
            o += static_cast<char>(0x80 | (cp & 0x3F));
        } else {
            o += static_cast<char>(0xF0 | (cp >> 18));
            o += static_cast<char>(0x80 | ((cp >> 12) & 0x3F));
            o += static_cast<char>(0x80 | ((cp >> 6) & 0x3F));
            o += static_cast<char>(0x80 | (cp & 0x3F));
        }
    }
    std::string string() {
        std::string o;
        i_++;
        while (i_ < s_.size() && s_[i_] != '"') {
            char c = s_[i_++];
            if (c != '\\') {
                o += c;
                continue;
            }
            if (i_ >= s_.size()) fail();
            char e = s_[i_++];
            switch (e) {
            case 'n': o += '\n'; break;
            case 't': o += '\t'; break;
            case 'r': o += '\r'; break;
            case 'b': o += '\b'; break;
            case 'f': o += '\f'; break;
            case 'u': {
                unsigned cp = hex4();
                if (cp >= 0xD800 && cp < 0xDC00 && s_.substr(i_, 2) == "\\u") {
                    i_ += 2;
                    unsigned lo = hex4();
                    cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
                }
                utf8(o, cp);
                break;
            }
            default: o += e;
            }
        }
        if (peek() != '"') fail();
        i_++;
        return o;
    }
    V value() {
        ws();
        V v;
        char c = peek();
        if (c == '{' || c == '[') {
            bool obj = c == '{';
            char end = obj ? '}' : ']';
            v.kind = obj ? V::Obj : V::Arr;
            i_++;
            ws();
            if (peek() == end) {
                i_++;
                return v;
            }
            for (;;) {
                ws();
                if (obj) {
                    if (peek() != '"') fail();
                    v.keys.push_back(string());
                    ws();
                    if (peek() != ':') fail();
                    i_++;
                }
                v.items.push_back(value());
                ws();
                if (peek() == ',') {
                    i_++;
                    continue;
                }
                if (peek() == end) {
                    i_++;
                    return v;
                }
                fail();
            }
        }
        if (c == '"') {
            v.kind = V::Str;
            v.s = string();
            return v;
        }
        if (s_.substr(i_, 4) == "null") {
            i_ += 4;
            return v;
        }
        if (s_.substr(i_, 4) == "true") {
            i_ += 4;
            v.kind = V::Bool;
            v.b = true;
            return v;
        }
        if (s_.substr(i_, 5) == "false") {
            i_ += 5;
            v.kind = V::Bool;
            return v;
        }
        std::size_t start = i_;
        while (i_ < s_.size() && std::string_view("+-.eE0123456789").find(s_[i_]) != std::string_view::npos) i_++;
        if (i_ == start) fail();
        v.kind = V::Num;
        v.s = std::string(s_.substr(start, i_ - start));
        return v;
    }
};

inline V parse(std::string_view s) { return Parser(s).parse(); }

template <class T>
struct From;

template <>
struct From<std::int64_t> {
    static std::int64_t get(const V& v) { return std::stoll(v.s); }
};
template <>
struct From<std::size_t> {
    static std::size_t get(const V& v) { return static_cast<std::size_t>(std::stoull(v.s)); }
};
template <>
struct From<bool> {
    static bool get(const V& v) { return v.b; }
};
template <>
struct From<std::string> {
    static std::string get(const V& v) { return v.s; }
};
template <class T>
struct From<std::vector<T>> {
    static std::vector<T> get(const V& v) {
        std::vector<T> out;
        out.reserve(v.items.size());
        for (const auto& x : v.items) out.push_back(From<T>::get(x));
        return out;
    }
};

template <class T>
T from(const V& v) {
    return From<T>::get(v);
}

inline void quote(std::string& o, std::string_view s) {
    o += '"';
    for (unsigned char c : s) {
        if (c == '"' || c == '\\') {
            o += '\\';
            o += static_cast<char>(c);
        } else if (c == '\n') {
            o += "\\n";
        } else if (c == '\t') {
            o += "\\t";
        } else if (c == '\r') {
            o += "\\r";
        } else if (c < 0x20) {
            char buf[8];
            std::snprintf(buf, sizeof buf, "\\u%04x", c);
            o += buf;
        } else {
            o += static_cast<char>(c);
        }
    }
    o += '"';
}

inline void put(std::string& o, bool b);
template <std::integral I>
    requires(!std::same_as<I, bool>)
void put(std::string& o, I x);
inline void put(std::string& o, std::string_view s);
inline void put(std::string& o, const std::string& s);
template <class T>
void put(std::string& o, const std::optional<T>& x);
template <class T>
void put(std::string& o, const std::vector<T>& xs);
template <class T, std::size_t N>
void put(std::string& o, std::span<T, N> xs);

inline void put(std::string& o, bool b) { o += b ? "true" : "false"; }
template <std::integral I>
    requires(!std::same_as<I, bool>)
void put(std::string& o, I x) {
    o += std::to_string(x);
}
inline void put(std::string& o, std::string_view s) { quote(o, s); }
inline void put(std::string& o, const std::string& s) { quote(o, s); }
template <class T>
void put(std::string& o, const std::optional<T>& x) {
    if (x) put(o, *x);
    else o += "null";
}
template <class T>
void put(std::string& o, const std::vector<T>& xs) {
    o += '[';
    for (std::size_t i = 0; i < xs.size(); i++) {
        if (i) o += ',';
        if constexpr (std::same_as<T, bool>) put(o, static_cast<bool>(xs[i]));
        else put(o, xs[i]);
    }
    o += ']';
}
template <class T, std::size_t N>
void put(std::string& o, std::span<T, N> xs) {
    o += '[';
    for (std::size_t i = 0; i < xs.size(); i++) {
        if (i) o += ',';
        put(o, xs[i]);
    }
    o += ']';
}

}  // namespace algo_json
