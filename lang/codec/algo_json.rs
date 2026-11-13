// SPDX-License-Identifier: MIT
// A small JSON reader and writer for the Rust adapters of https://algorithm.tamnd.com (spec 09.2).
#![allow(dead_code)]

use std::fmt::Write;

#[derive(Debug, Clone)]
pub enum V {
    Null,
    Bool(bool),
    Num(String),
    Str(String),
    Arr(Vec<V>),
    Obj(Vec<(String, V)>),
}

impl V {
    pub fn get(&self, key: &str) -> Result<&V, String> {
        if let V::Obj(m) = self {
            if let Some((_, v)) = m.iter().find(|(k, _)| k == key) {
                return Ok(v);
            }
        }
        Err(format!("no key {key}"))
    }
    pub fn as_str(&self) -> Option<&str> {
        if let V::Str(s) = self { Some(s) } else { None }
    }
}

pub fn parse(s: &str) -> Result<V, String> {
    let b = s.as_bytes();
    let mut i = 0;
    let v = value(b, &mut i)?;
    ws(b, &mut i);
    if i != b.len() {
        return Err("extra text after the JSON value".into());
    }
    Ok(v)
}

fn ws(b: &[u8], i: &mut usize) {
    while *i < b.len() && matches!(b[*i], b' ' | b'\t' | b'\n' | b'\r') {
        *i += 1;
    }
}

fn hex4(b: &[u8], i: &mut usize) -> Result<u32, String> {
    let h = b.get(*i..*i + 4).ok_or("short escape")?;
    let h = std::str::from_utf8(h).map_err(|e| e.to_string())?;
    *i += 4;
    u32::from_str_radix(h, 16).map_err(|e| e.to_string())
}

fn string(b: &[u8], i: &mut usize) -> Result<String, String> {
    *i += 1;
    let mut out: Vec<u8> = Vec::new();
    while *i < b.len() {
        let c = b[*i];
        *i += 1;
        match c {
            b'"' => return String::from_utf8(out).map_err(|e| e.to_string()),
            b'\\' => {
                let e = *b.get(*i).ok_or("short escape")?;
                *i += 1;
                match e {
                    b'n' => out.push(b'\n'),
                    b't' => out.push(b'\t'),
                    b'r' => out.push(b'\r'),
                    b'b' => out.push(8),
                    b'f' => out.push(12),
                    b'u' => {
                        let mut cp = hex4(b, i)?;
                        if (0xD800..0xDC00).contains(&cp) && b[*i..].starts_with(b"\\u") {
                            *i += 2;
                            let lo = hex4(b, i)?;
                            cp = 0x10000 + ((cp - 0xD800) << 10) + (lo.wrapping_sub(0xDC00));
                        }
                        let ch = char::from_u32(cp).ok_or("bad escape")?;
                        let mut buf = [0u8; 4];
                        out.extend_from_slice(ch.encode_utf8(&mut buf).as_bytes());
                    }
                    other => out.push(other),
                }
            }
            _ => out.push(c),
        }
    }
    Err("the string is not closed".into())
}

fn value(b: &[u8], i: &mut usize) -> Result<V, String> {
    ws(b, i);
    let c = *b.get(*i).ok_or("end of input")?;
    match c {
        b'{' | b'[' => {
            let obj = c == b'{';
            let end = if obj { b'}' } else { b']' };
            *i += 1;
            let mut items: Vec<V> = Vec::new();
            let mut keys: Vec<String> = Vec::new();
            ws(b, i);
            if b.get(*i) == Some(&end) {
                *i += 1;
            } else {
                loop {
                    ws(b, i);
                    if obj {
                        if b.get(*i) != Some(&b'"') {
                            return Err("a key is not a string".into());
                        }
                        keys.push(string(b, i)?);
                        ws(b, i);
                        if b.get(*i) != Some(&b':') {
                            return Err("no colon after a key".into());
                        }
                        *i += 1;
                    }
                    items.push(value(b, i)?);
                    ws(b, i);
                    match b.get(*i) {
                        Some(b',') => *i += 1,
                        Some(x) if *x == end => {
                            *i += 1;
                            break;
                        }
                        _ => return Err("a list or an object is not closed".into()),
                    }
                }
            }
            if obj { Ok(V::Obj(keys.into_iter().zip(items).collect())) } else { Ok(V::Arr(items)) }
        }
        b'"' => Ok(V::Str(string(b, i)?)),
        b'n' if b[*i..].starts_with(b"null") => {
            *i += 4;
            Ok(V::Null)
        }
        b't' if b[*i..].starts_with(b"true") => {
            *i += 4;
            Ok(V::Bool(true))
        }
        b'f' if b[*i..].starts_with(b"false") => {
            *i += 5;
            Ok(V::Bool(false))
        }
        _ => {
            let start = *i;
            while *i < b.len() && matches!(b[*i], b'-' | b'+' | b'.' | b'e' | b'E' | b'0'..=b'9') {
                *i += 1;
            }
            if *i == start {
                return Err("not a JSON value".into());
            }
            Ok(V::Num(String::from_utf8_lossy(&b[start..*i]).into_owned()))
        }
    }
}

pub trait FromJ: Sized {
    fn from_j(v: &V) -> Result<Self, String>;
}

macro_rules! from_int {
    ($($t:ty),*) => {$(
        impl FromJ for $t {
            fn from_j(v: &V) -> Result<Self, String> {
                match v {
                    V::Num(s) | V::Str(s) => s.parse::<$t>().map_err(|e| format!("{s}: {e}")),
                    _ => Err("not an integer".into()),
                }
            }
        }
    )*};
}
from_int!(i64, i32, usize, u64);

impl FromJ for bool {
    fn from_j(v: &V) -> Result<Self, String> {
        if let V::Bool(b) = v { Ok(*b) } else { Err("not a bool".into()) }
    }
}

impl FromJ for String {
    fn from_j(v: &V) -> Result<Self, String> {
        if let V::Str(s) = v { Ok(s.clone()) } else { Err("not a string".into()) }
    }
}

impl<T: FromJ> FromJ for Vec<T> {
    fn from_j(v: &V) -> Result<Self, String> {
        if let V::Arr(xs) = v { xs.iter().map(T::from_j).collect() } else { Err("not a list".into()) }
    }
}

pub fn quote(s: &str) -> String {
    let mut o = String::with_capacity(s.len() + 2);
    o.push('"');
    for c in s.chars() {
        match c {
            '"' => o.push_str("\\\""),
            '\\' => o.push_str("\\\\"),
            '\n' => o.push_str("\\n"),
            '\t' => o.push_str("\\t"),
            '\r' => o.push_str("\\r"),
            c if (c as u32) < 0x20 => {
                let _ = write!(o, "\\u{:04x}", c as u32);
            }
            c => o.push(c),
        }
    }
    o.push('"');
    o
}

pub trait ToJ {
    fn to_j(&self, o: &mut String);
}

macro_rules! to_int {
    ($($t:ty),*) => {$(
        impl ToJ for $t {
            fn to_j(&self, o: &mut String) {
                let _ = write!(o, "{}", self);
            }
        }
    )*};
}
to_int!(i8, i16, i32, i64, i128, isize, u8, u16, u32, u64, u128, usize);

impl ToJ for bool {
    fn to_j(&self, o: &mut String) {
        o.push_str(if *self { "true" } else { "false" });
    }
}

impl ToJ for str {
    fn to_j(&self, o: &mut String) {
        o.push_str(&quote(self));
    }
}

impl ToJ for String {
    fn to_j(&self, o: &mut String) {
        o.push_str(&quote(self));
    }
}

impl ToJ for () {
    fn to_j(&self, o: &mut String) {
        o.push_str("null");
    }
}

impl<T: ToJ + ?Sized> ToJ for &T {
    fn to_j(&self, o: &mut String) {
        (**self).to_j(o);
    }
}

impl<T: ToJ> ToJ for Option<T> {
    fn to_j(&self, o: &mut String) {
        match self {
            Some(x) => x.to_j(o),
            None => o.push_str("null"),
        }
    }
}

impl<T: ToJ> ToJ for [T] {
    fn to_j(&self, o: &mut String) {
        o.push('[');
        for (i, x) in self.iter().enumerate() {
            if i > 0 {
                o.push(',');
            }
            x.to_j(o);
        }
        o.push(']');
    }
}

impl<T: ToJ> ToJ for Vec<T> {
    fn to_j(&self, o: &mut String) {
        self.as_slice().to_j(o);
    }
}
