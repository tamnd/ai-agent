// SPDX-License-Identifier: MIT
// A small JSON reader and writer for the Kotlin adapters of https://algorithm.tamnd.com (spec 09.2).

/** A number keeps its text, so a Long does not lose digits. */
class AlgoNum(val text: String)

object AlgoJson {
    fun parse(s: String): Any? {
        val p = P(s)
        val v = p.value()
        p.ws()
        require(p.i == s.length) { "extra text after the JSON value" }
        return v
    }

    private class P(val s: String) {
        var i = 0

        fun ws() {
            while (i < s.length && s[i] in " \t\n\r") i++
        }

        fun peek(): Char = if (i < s.length) s[i] else '\u0000'

        fun value(): Any? {
            ws()
            val c = peek()
            when {
                c == '{' -> {
                    i++
                    val m = LinkedHashMap<String, Any?>()
                    ws()
                    if (peek() == '}') { i++; return m }
                    while (true) {
                        ws()
                        val k = string()
                        ws()
                        require(peek() == ':') { "no colon after a key" }
                        i++
                        m[k] = value()
                        ws()
                        when (peek()) {
                            ',' -> i++
                            '}' -> { i++; return m }
                            else -> throw IllegalArgumentException("the object is not closed")
                        }
                    }
                }
                c == '[' -> {
                    i++
                    val xs = ArrayList<Any?>()
                    ws()
                    if (peek() == ']') { i++; return xs }
                    while (true) {
                        xs.add(value())
                        ws()
                        when (peek()) {
                            ',' -> i++
                            ']' -> { i++; return xs }
                            else -> throw IllegalArgumentException("the list is not closed")
                        }
                    }
                }
                c == '"' -> return string()
                s.startsWith("null", i) -> { i += 4; return null }
                s.startsWith("true", i) -> { i += 4; return true }
                s.startsWith("false", i) -> { i += 5; return false }
            }
            val start = i
            while (i < s.length && s[i] in "+-.eE0123456789") i++
            require(i > start) { "not a JSON value" }
            return AlgoNum(s.substring(start, i))
        }

        fun string(): String {
            require(peek() == '"') { "not a string" }
            i++
            val o = StringBuilder()
            while (i < s.length && s[i] != '"') {
                val c = s[i++]
                if (c != '\\') { o.append(c); continue }
                when (val e = s[i++]) {
                    'n' -> o.append('\n')
                    't' -> o.append('\t')
                    'r' -> o.append('\r')
                    'b' -> o.append('\b')
                    'f' -> o.append('\u000C')
                    'u' -> { o.append(s.substring(i, i + 4).toInt(16).toChar()); i += 4 }
                    else -> o.append(e)
                }
            }
            require(peek() == '"') { "the string is not closed" }
            i++
            return o.toString()
        }
    }

    fun toLong(v: Any?): Long = when (v) {
        is AlgoNum -> v.text.toLong()
        is String -> v.toLong()
        else -> throw IllegalArgumentException("not an integer")
    }

    fun toInt(v: Any?): Int = Math.toIntExact(toLong(v))

    fun toBool(v: Any?): Boolean = v as Boolean

    fun str(v: Any?): String = v as String

    fun list(v: Any?): List<Any?> = v as List<Any?>

    fun quote(o: StringBuilder, t: String) {
        o.append('"')
        for (c in t) {
            when {
                c == '"' -> o.append("\\\"")
                c == '\\' -> o.append("\\\\")
                c == '\n' -> o.append("\\n")
                c == '\t' -> o.append("\\t")
                c == '\r' -> o.append("\\r")
                c < ' ' -> o.append(String.format("\\u%04x", c.code))
                else -> o.append(c)
            }
        }
        o.append('"')
    }

    fun quoted(t: String): String = StringBuilder().also { quote(it, t) }.toString()

    fun put(o: StringBuilder, v: Any?) {
        when (v) {
            null, is Unit -> o.append("null")
            is Boolean -> o.append(if (v) "true" else "false")
            is Number -> o.append(v.toString())
            is String -> quote(o, v)
            is LongArray -> put(o, v.toList())
            is IntArray -> put(o, v.toList())
            is BooleanArray -> put(o, v.toList())
            is Array<*> -> put(o, v.toList())
            is Iterable<*> -> {
                o.append('[')
                var first = true
                for (x in v) {
                    if (!first) o.append(',')
                    first = false
                    put(o, x)
                }
                o.append(']')
            }
            else -> quote(o, v.toString())
        }
    }
}
