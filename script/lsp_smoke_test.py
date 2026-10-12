#!/usr/bin/env python3
"""Exercise the built sqls binary using real, framed stdio LSP messages.

Run: go build -o sqls . && python3 script/lsp_smoke_test.py
Only Python's standard library and SQLite are needed.
"""
import argparse
import json
from pathlib import Path
import queue
import sqlite3
import subprocess
import tempfile
import threading
import unittest

ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / "sqls"
URI = "file:///sqls-test/query.sql"


def create_database(path):
    with sqlite3.connect(path) as db:
        db.executescript("""
            CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT);
            CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id), total REAL);
            INSERT INTO users VALUES (1, 'Ada', 'ada@example.com');
            INSERT INTO orders VALUES (1, 1, 42.5);
        """)


class Client:
    def __init__(self, binary, config):
        self.process = subprocess.Popen([str(binary), "-config", str(config)],
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE)
        self.messages = queue.Queue()
        self.notifications = []
        self.stderr = []
        self.next_id = 0
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()
        self.logger = threading.Thread(target=self._log, daemon=True)
        self.logger.start()

    def _log(self):
        for line in self.process.stderr:
            self.stderr.append(line.decode(errors="replace"))

    def _read(self):
        try:
            while True:
                headers = {}
                while True:
                    line = self.process.stdout.readline()
                    if not line:
                        raise EOFError("sqls closed stdout")
                    if line == b"\r\n":
                        break
                    key, value = line.decode().split(":", 1)
                    headers[key.lower()] = value.strip()
                length = int(headers["content-length"])
                body = self.process.stdout.read(length)
                if len(body) != length:
                    raise EOFError("truncated LSP message")
                self.messages.put(json.loads(body))
        except Exception as error:
            self.messages.put(error)

    def send(self, method, params=None, request=False):
        message = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            message["params"] = params
        if request:
            self.next_id += 1
            message["id"] = self.next_id
        body = json.dumps(message, ensure_ascii=False).encode()
        self.process.stdin.write(f"Content-Length: {len(body)}\r\n\r\n".encode() + body)
        self.process.stdin.flush()
        return message.get("id")

    def receive(self):
        message = self.messages.get(timeout=10)
        if isinstance(message, Exception):
            raise message
        return message

    def response(self, method, params=None):
        request_id = self.send(method, params, request=True)
        while True:
            message = self.receive()
            if message.get("id") == request_id:
                return message
            if "id" in message:
                raise AssertionError(f"unexpected server request: {message}")
            self.notifications.append(message)

    def call(self, method, params=None):
        response = self.response(method, params)
        if "error" in response:
            raise AssertionError(response["error"])
        return response.get("result")

    def diagnostic(self):
        while True:
            for i, message in enumerate(self.notifications):
                if message.get("method") == "textDocument/publishDiagnostics":
                    return self.notifications.pop(i)["params"]
            self.notifications.append(self.receive())

    def close(self):
        if self.process.poll() is None:
            self.process.kill()
        self.process.wait(timeout=5)
        self.reader.join(timeout=5)
        self.logger.join(timeout=5)
        for stream in (self.process.stdin, self.process.stdout, self.process.stderr):
            stream.close()


def position(text, offset):
    prefix = text[:offset]
    return {"line": prefix.count("\n"),
            "character": len(prefix.rsplit("\n", 1)[-1].encode("utf-16-le")) // 2}


def offset(text, pos):
    lines = text.splitlines(keepends=True)
    if pos["line"] == len(lines) and pos["character"] == 0:
        return len(text)
    line = lines[pos["line"]] if lines else ""
    units = 0
    for i, char in enumerate(line):
        if units == pos["character"]:
            return sum(map(len, lines[:pos["line"]])) + i
        units += len(char.encode("utf-16-le")) // 2
    if units == pos["character"]:
        return sum(map(len, lines[:pos["line"]])) + len(line)
    raise AssertionError(f"invalid UTF-16 position: {pos}")


def apply_edits(text, edits):
    for edit in sorted(edits, key=lambda e: (e["range"]["start"]["line"],
                                            e["range"]["start"]["character"]), reverse=True):
        start = offset(text, edit["range"]["start"])
        end = offset(text, edit["range"]["end"])
        text = text[:start] + edit["newText"] + text[end:]
    return text


class LSPTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="sqls-lsp-")
        self.addCleanup(self.temp.cleanup)
        db = Path(self.temp.name) / "fixture.sqlite"
        create_database(db)
        config = Path(self.temp.name) / "config.yml"
        config.write_text(f"connections:\n  - driver: sqlite3\n    dataSourceName: {json.dumps(str(db))}\n")
        self.client = Client(BINARY, config)
        self.addCleanup(self.client.close)

    def initialize(self):
        result = self.client.call("initialize", {
            "processId": None, "rootUri": None,
            "capabilities": {"workspace": {"workspaceEdit": {"documentChanges": True}}}})
        self.client.send("initialized", {})
        return result["capabilities"]

    def open(self, text, version=7):
        self.client.send("textDocument/didOpen", {"textDocument": {
            "uri": URI, "languageId": "sql", "version": version, "text": text}})
        return self.client.diagnostic()

    def at(self, text, marker):
        return {"textDocument": {"uri": URI}, "position": position(text, text.index(marker))}

    def format(self, text):
        return self.client.call("textDocument/formatting", {
            "textDocument": {"uri": URI}, "options": {"tabSize": 2, "insertSpaces": True}})

    def test_capabilities_and_keyword_completion(self):
        self.assertEqual(self.initialize()["textDocumentSync"], 1)
        self.open("")
        items = self.client.call("textDocument/completion", {
            "textDocument": {"uri": URI}, "position": {"line": 0, "character": 0}})
        self.assertIn("SELECT", [item["label"].upper() for item in items])

    def test_table_and_column_completion(self):
        self.initialize()
        text = "SELECT u. FROM users AS u;"
        self.open(text)
        params = self.at(text, ".")
        params["position"]["character"] += 1
        items = self.client.call("textDocument/completion", params)
        self.assertTrue({"id", "name", "email"}.issubset({i["label"] for i in items}))
        text = "SELECT * FROM "
        self.client.send("textDocument/didChange", {"textDocument": {"uri": URI, "version": 8},
                                                 "contentChanges": [{"text": text}]})
        self.client.diagnostic()
        params["position"] = position(text, len(text))
        items = self.client.call("textDocument/completion", params)
        self.assertTrue({"users", "orders"}.issubset({i["label"] for i in items}))

    def test_hover_at_first_character(self):
        self.initialize()
        text = "SELECT name FROM users;"
        self.open(text)
        result = self.client.call("textDocument/hover", self.at(text, "name"))
        self.assertIsNotNone(result)
        self.assertIn("TEXT", result["contents"]["value"])
        self.assertEqual(result["range"]["start"], position(text, text.index("name")))

    def test_hover_does_not_select_next_token(self):
        self.initialize()
        text = "SELECT name FROM users;"
        self.open(text)
        params = self.at(text, "name")
        params["position"]["character"] -= 1
        self.assertIsNone(self.client.call("textDocument/hover", params))

    def test_definition_at_last_alias_character(self):
        self.initialize()
        text = "SELECT u.name FROM users AS u;"
        self.open(text)
        result = self.client.call("textDocument/definition", self.at(text, "u.name"))
        self.assertTrue(result)
        self.assertEqual(result[0]["range"]["start"], position(text, text.rindex("u;")))

    def test_rename_uses_document_version(self):
        self.initialize()
        text = "SELECT u.name FROM users AS u;"
        self.open(text, version=7)
        params = self.at(text, "u.name")
        params["newName"] = "customer"
        result = self.client.call("textDocument/rename", params)
        change = result["documentChanges"][0]
        self.assertEqual(change["textDocument"]["version"], 7)
        self.assertEqual(apply_edits(text, change["edits"]), "SELECT customer.name FROM users AS customer;")

    def test_unicode_rename_ranges(self):
        self.initialize()
        text = "SELECT '😀', u.name FROM users AS u;"
        self.open(text)
        params = self.at(text, "u.name")
        params["newName"] = "customer"
        result = self.client.call("textDocument/rename", params)
        self.assertIsNotNone(result)
        self.assertEqual(apply_edits(text, result["documentChanges"][0]["edits"]),
                         "SELECT '😀', customer.name FROM users AS customer;")

    def test_formatting_applies_and_is_idempotent(self):
        self.initialize()
        text = "  select   name,email from users;  \n"
        self.open(text)
        formatted = apply_edits(text, self.format(text))
        self.assertTrue(formatted.startswith("SELECT"), repr(formatted))
        self.client.send("textDocument/didChange", {"textDocument": {"uri": URI, "version": 8},
                                                 "contentChanges": [{"text": formatted}]})
        self.client.diagnostic()
        self.assertEqual(apply_edits(formatted, self.format(formatted) or []), formatted)

    def test_empty_document_formatting(self):
        self.initialize()
        self.open("")
        self.assertFalse(self.format(""))

    def test_range_formatting(self):
        self.initialize()
        text = "select 1;\nselect   name from users;\nselect 2;"
        self.open(text)
        result = self.client.call("textDocument/rangeFormatting", {
            "textDocument": {"uri": URI},
            "range": {"start": {"line": 1, "character": 0}, "end": {"line": 1, "character": 26}},
            "options": {"tabSize": 2, "insertSpaces": True}})
        self.assertTrue(result)
        formatted = apply_edits(text, result)
        self.assertTrue(formatted.startswith("select 1;\nSELECT"), repr(formatted))
        self.assertTrue(formatted.endswith("\nselect 2;"), repr(formatted))

    def test_diagnostics_change_and_close(self):
        self.initialize()
        diagnostics = self.open("SELECT * FROM users;")["diagnostics"]
        self.assertTrue(diagnostics)
        self.client.send("textDocument/didChange", {"textDocument": {"uri": URI, "version": 8},
                                                 "contentChanges": [{"text": "SELECT name FROM users;"}]})
        self.assertEqual(self.client.diagnostic()["diagnostics"], [])
        self.client.send("textDocument/didClose", {"textDocument": {"uri": URI}})
        self.assertEqual(self.client.diagnostic()["diagnostics"], [])

    def test_signature_help(self):
        self.initialize()
        text = "INSERT INTO users (name, email) VALUES ('Ada', );"
        self.open(text)
        params = self.at(text, ");")
        result = self.client.call("textDocument/signatureHelp", params)
        self.assertIsNotNone(result)
        self.assertEqual(result["activeParameter"], 1)
        self.assertEqual([p["label"] for p in result["signatures"][0]["parameters"]], ["name", "email"])

    def test_missing_params(self):
        self.initialize()
        response = self.client.response("workspace/didChangeConfiguration")
        self.assertEqual(response["error"]["code"], -32602)
        self.assertNotIn("panic", "".join(self.client.stderr))

    def test_unknown_method(self):
        self.initialize()
        self.assertEqual(self.client.response("sqls/unknown", {})["error"]["code"], -32601)

    def test_rename_after_change(self):
        self.initialize()
        text = "SELECT u.name FROM users AS u;"
        self.open(text)
        self.client.send("textDocument/didChange", {"textDocument": {"uri": URI, "version": 12},
                                                 "contentChanges": [{"text": text}]})
        self.client.diagnostic()
        result = self.client.call("textDocument/rename", {**self.at(text, "u.name"), "newName": "customer"})
        self.assertEqual(result["documentChanges"][0]["textDocument"]["version"], 12)

    def test_rename_version_boundaries(self):
        self.initialize()
        text = "SELECT u.name FROM users AS u;"
        for version in (-2147483648, 2147483647):
            with self.subTest(version=version):
                self.open(text, version=version)
                result = self.client.call("textDocument/rename", {**self.at(text, "u.name"), "newName": "customer"})
                self.assertEqual(result["documentChanges"][0]["textDocument"]["version"], version)
        for version in (-2147483649, 2147483648):
            with self.subTest(version=version):
                response = self.client.response("textDocument/didChange", {
                    "textDocument": {"uri": URI, "version": version}, "contentChanges": [{"text": text}]})
                self.assertEqual(response["error"]["code"], -32602)

    def test_requests_after_shutdown(self):
        self.initialize()
        self.client.call("shutdown")
        self.assertEqual(self.client.response("textDocument/completion", {})["error"]["code"], -32600)

    def test_unicode_diagnostics(self):
        self.initialize()
        text = "SELECT '😀', * FROM users;"
        diagnostics = self.open(text)["diagnostics"]
        stars = [d for d in diagnostics if 'SELECT *' in d["message"]]
        self.assertTrue(stars)
        self.assertEqual(stars[0]["range"]["start"], position(text, text.index('*')))

    def test_where_diagnostics_respect_sqlite(self):
        self.initialize()
        text = "SELECT u.name, u.email\nFROM users AS u\nWHERE u.id ;"
        self.assertEqual(self.open(text)["diagnostics"], [])
        invalid = text.replace("u.id ;", "u.id = ;")
        self.client.send("textDocument/didChange", {"textDocument": {"uri": URI, "version": 8},
                                                 "contentChanges": [{"text": invalid}]})
        diagnostics = self.client.diagnostic()["diagnostics"]
        syntax = [d for d in diagnostics if d["code"] == "syntax-error"]
        self.assertEqual(len(syntax), 1)
        self.assertEqual(syntax[0]["severity"], 1)
        self.assertEqual(syntax[0]["range"]["start"], {"line": 2, "character": 11})
        self.client.send("textDocument/didChange", {"textDocument": {"uri": URI, "version": 9},
                                                 "contentChanges": [{"text": text}]})
        self.assertEqual(self.client.diagnostic()["diagnostics"], [])

    def test_formatting_preserves_escaped_quotes(self):
        self.initialize()
        text = "select 'O''Reilly', name from users;"
        self.open(text)
        formatted = apply_edits(text, self.format(text))
        self.assertIn("'O''Reilly'", formatted)

    def test_rename_after_multiline_string(self):
        self.initialize()
        text = "SELECT '😀\nhello', u.name FROM users AS u;"
        self.open(text)
        result = self.client.call("textDocument/rename", {**self.at(text, "u.name"), "newName": "customer"})
        self.assertIsNotNone(result)
        self.assertEqual(apply_edits(text, result["documentChanges"][0]["edits"]),
                         "SELECT '😀\nhello', customer.name FROM users AS customer;")

    def test_shutdown_exit(self):
        self.initialize()
        self.assertIsNone(self.client.call("shutdown"))
        self.client.send("exit")
        # Keep stdin open: exit must terminate without waiting for EOF.
        self.assertEqual(self.client.process.wait(timeout=3), 0)
        self.client.logger.join(timeout=3)
        self.assertNotIn("panic", "".join(self.client.stderr))

    def test_exit_without_shutdown(self):
        self.client.send("exit")
        self.assertEqual(self.client.process.wait(timeout=3), 1)

    def test_request_before_initialize(self):
        self.assertEqual(self.client.response("textDocument/completion", {})["error"]["code"], -32002)
        self.initialize()

    def test_duplicate_initialize(self):
        self.initialize()
        self.assertEqual(self.client.response("initialize", {})["error"]["code"], -32600)

    def test_invalid_param_types(self):
        self.initialize()
        self.assertEqual(self.client.response("textDocument/formatting", {"textDocument": 42})["error"]["code"], -32602)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, default=BINARY)
    args, remaining = parser.parse_known_args()
    BINARY = args.binary.resolve()
    unittest.main(argv=[__file__, *remaining], verbosity=2)
