// SPDX-License-Identifier: MIT
// A small JSON reader and writer for the Java adapters of https://algorithm.tamnd.com (spec 09.2).
import java.lang.reflect.Array;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.OptionalInt;
import java.util.OptionalLong;

final class AlgoJson {
    /** A number keeps its text, so a long does not lose digits. */
    record Num(String text) {}

    private final String s;
    private int i;

    private AlgoJson(String s) {
        this.s = s;
    }

    static Object parse(String s) {
        AlgoJson p = new AlgoJson(s);
        Object v = p.value();
        p.ws();
        if (p.i != s.length()) throw new IllegalArgumentException("extra text after the JSON value");
        return v;
    }

    private void ws() {
        while (i < s.length() && " \t\n\r".indexOf(s.charAt(i)) >= 0) i++;
    }

    private char peek() {
        return i < s.length() ? s.charAt(i) : '\0';
    }

    private Object value() {
        ws();
        char c = peek();
        if (c == '{') {
            i++;
            Map<String, Object> m = new LinkedHashMap<>();
            ws();
            if (peek() == '}') {
                i++;
                return m;
            }
            for (;;) {
                ws();
                String k = string();
                ws();
                if (peek() != ':') throw new IllegalArgumentException("no colon after a key");
                i++;
                m.put(k, value());
                ws();
                if (peek() == ',') { i++; continue; }
                if (peek() == '}') { i++; return m; }
                throw new IllegalArgumentException("the object is not closed");
            }
        }
        if (c == '[') {
            i++;
            List<Object> xs = new ArrayList<>();
            ws();
            if (peek() == ']') {
                i++;
                return xs;
            }
            for (;;) {
                xs.add(value());
                ws();
                if (peek() == ',') { i++; continue; }
                if (peek() == ']') { i++; return xs; }
                throw new IllegalArgumentException("the list is not closed");
            }
        }
        if (c == '"') return string();
        if (s.startsWith("null", i)) { i += 4; return null; }
        if (s.startsWith("true", i)) { i += 4; return Boolean.TRUE; }
        if (s.startsWith("false", i)) { i += 5; return Boolean.FALSE; }
        int start = i;
        while (i < s.length() && "+-.eE0123456789".indexOf(s.charAt(i)) >= 0) i++;
        if (i == start) throw new IllegalArgumentException("not a JSON value");
        return new Num(s.substring(start, i));
    }

    private String string() {
        if (peek() != '"') throw new IllegalArgumentException("not a string");
        i++;
        StringBuilder o = new StringBuilder();
        while (i < s.length() && s.charAt(i) != '"') {
            char c = s.charAt(i++);
            if (c != '\\') { o.append(c); continue; }
            char e = s.charAt(i++);
            switch (e) {
                case 'n' -> o.append('\n');
                case 't' -> o.append('\t');
                case 'r' -> o.append('\r');
                case 'b' -> o.append('\b');
                case 'f' -> o.append('\f');
                case 'u' -> { o.append((char) Integer.parseInt(s.substring(i, i + 4), 16)); i += 4; }
                default -> o.append(e);
            }
        }
        if (peek() != '"') throw new IllegalArgumentException("the string is not closed");
        i++;
        return o.toString();
    }

    static long toLong(Object v) {
        if (v instanceof Num n) return Long.parseLong(n.text());
        if (v instanceof String t) return Long.parseLong(t);
        throw new IllegalArgumentException("not an integer");
    }

    static int toInt(Object v) {
        return Math.toIntExact(toLong(v));
    }

    static boolean toBool(Object v) {
        return (Boolean) v;
    }

    static String str(Object v) {
        return (String) v;
    }

    static List<?> list(Object v) {
        return (List<?>) v;
    }

    static long[] toLongArray(Object v) {
        List<?> xs = list(v);
        long[] a = new long[xs.size()];
        for (int k = 0; k < a.length; k++) a[k] = toLong(xs.get(k));
        return a;
    }

    static int[] toIntArray(Object v) {
        List<?> xs = list(v);
        int[] a = new int[xs.size()];
        for (int k = 0; k < a.length; k++) a[k] = toInt(xs.get(k));
        return a;
    }

    static boolean[] toBoolArray(Object v) {
        List<?> xs = list(v);
        boolean[] a = new boolean[xs.size()];
        for (int k = 0; k < a.length; k++) a[k] = toBool(xs.get(k));
        return a;
    }

    static String[] toStringArray(Object v) {
        List<?> xs = list(v);
        String[] a = new String[xs.size()];
        for (int k = 0; k < a.length; k++) a[k] = str(xs.get(k));
        return a;
    }

    static void quote(StringBuilder o, String t) {
        o.append('"');
        for (int k = 0; k < t.length(); k++) {
            char c = t.charAt(k);
            switch (c) {
                case '"' -> o.append("\\\"");
                case '\\' -> o.append("\\\\");
                case '\n' -> o.append("\\n");
                case '\t' -> o.append("\\t");
                case '\r' -> o.append("\\r");
                default -> {
                    if (c < 0x20) o.append(String.format("\\u%04x", (int) c));
                    else o.append(c);
                }
            }
        }
        o.append('"');
    }

    static void put(StringBuilder o, Object v) {
        if (v == null) o.append("null");
        else if (v instanceof Boolean b) o.append(b ? "true" : "false");
        else if (v instanceof Byte || v instanceof Short || v instanceof Integer || v instanceof Long) o.append(v);
        else if (v instanceof Number n) o.append(n.toString());
        else if (v instanceof String t) quote(o, t);
        else if (v instanceof OptionalInt x) { if (x.isPresent()) o.append(x.getAsInt()); else o.append("null"); }
        else if (v instanceof OptionalLong x) { if (x.isPresent()) o.append(x.getAsLong()); else o.append("null"); }
        else if (v instanceof Optional<?> x) put(o, x.orElse(null));
        else if (v instanceof Iterable<?> xs) {
            o.append('[');
            boolean first = true;
            for (Object x : xs) {
                if (!first) o.append(',');
                first = false;
                put(o, x);
            }
            o.append(']');
        } else if (v.getClass().isArray()) {
            o.append('[');
            for (int k = 0; k < Array.getLength(v); k++) {
                if (k > 0) o.append(',');
                put(o, Array.get(v, k));
            }
            o.append(']');
        } else quote(o, v.toString());
    }
}
